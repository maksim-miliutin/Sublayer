package capture

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"
)

var deviceRates = []int{8000, 22050, 44100, 48000, 96000}

func tone(rate int, hz, seconds, amplitude float64) []float32 {
	out := make([]float32, int(float64(rate)*seconds))
	for n := range out {
		out[n] = float32(amplitude * math.Sin(2*math.Pi*hz*float64(n)/float64(rate)))
	}

	return out
}

// Exactly one second of signal holds a whole number of periods of any whole-hertz
// tone, so the projection below sees no leakage from the cut.
func lastSecond(samples []float32) []float32 {
	return samples[len(samples)-Rate:]
}

func amplitudeAt(samples []float32, hz float64) float64 {
	var re, im float64
	for n, s := range samples {
		phase := 2 * math.Pi * hz * float64(n) / Rate
		re += float64(s) * math.Cos(phase)
		im += float64(s) * math.Sin(phase)
	}

	return 2 * math.Hypot(re, im) / float64(len(samples))
}

func rms(samples []float32) float64 {
	var sum float64
	for _, s := range samples {
		sum += float64(s) * float64(s)
	}

	return math.Sqrt(sum / float64(len(samples)))
}

func TestResamplerKeepsSpeechTonesAtTheirLevel(t *testing.T) {
	for _, from := range deviceRates {
		for _, hz := range []float64{200, 1000, 3000, 5000} {
			if hz >= float64(from)/2 {
				continue
			}

			out := newResampler(from, Rate).feed(tone(from, hz, 1.5, 0.5))

			got := amplitudeAt(lastSecond(out), hz)
			if math.Abs(got-0.5) > 0.001 {
				t.Errorf("%d Hz: a %v Hz tone of 0.5 came out at %.4f", from, hz, got)
			}
		}
	}
}

func TestResamplerSuppressesWhatWouldFoldIntoSpeech(t *testing.T) {
	cases := []struct {
		from int
		hz   float64
	}{
		{48000, 12000},
		{48000, 20000},
		{44100, 10000},
		{44100, 15000},
		{96000, 30000},
	}

	for _, c := range cases {
		out := newResampler(c.from, Rate).feed(tone(c.from, c.hz, 1.5, 0.5))

		if level := rms(lastSecond(out)); level > 0.0001 {
			t.Errorf("%d Hz: a %v Hz tone of 0.5 left %.5f rms in the output", c.from, c.hz, level)
		}
	}
}

func TestResamplerGivesTheSameSamplesWhateverTheChunks(t *testing.T) {
	random := rand.New(rand.NewPCG(1, 2))
	noise := make([]float32, 3*44100)
	for i := range noise {
		noise[i] = random.Float32()*2 - 1
	}

	whole := newResampler(44100, Rate).feed(noise)

	chunked := newResampler(44100, Rate)
	var pieces []float32
	for rest := noise; len(rest) > 0; {
		n := min(random.IntN(700), len(rest))
		pieces = append(pieces, chunked.feed(rest[:n])...)
		rest = rest[n:]
	}

	if !slices.Equal(whole, pieces) {
		t.Errorf("chunked feeding gave %d samples that differ from %d fed at once", len(pieces), len(whole))
	}
}

func TestResamplerPassesASteadyLevelUnchanged(t *testing.T) {
	for _, from := range deviceRates {
		steady := make([]float32, from)
		for i := range steady {
			steady[i] = 0.25
		}

		out := newResampler(from, Rate).feed(steady)

		for i, s := range out[len(out)/2:] {
			if math.Abs(float64(s)-0.25) > 1e-6 {
				t.Fatalf("%d Hz: sample %d is %v, want 0.25", from, len(out)/2+i, s)
			}
		}
	}
}

func TestResamplerKeepsPaceWithTheInput(t *testing.T) {
	for _, from := range deviceRates {
		r := newResampler(from, Rate)

		var produced int
		for range 10 {
			produced += len(r.feed(make([]float32, from/10)))
		}

		if produced < Rate-Rate/100 || produced > Rate {
			t.Errorf("%d Hz: one second in gave %d samples out, want just under %d", from, produced, Rate)
		}
	}
}
