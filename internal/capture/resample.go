package capture

import "math"

const (
	Rate = 16000 // samples per second of every chunk a Source hands out

	zeroCrossings = 16
	passband      = 0.9
)

type resampler struct {
	up, down int64
	phases   [][]float32
	held     []float32
	start    int64
	next     int64
}

func newResampler(from, to int) *resampler {
	g := gcd(from, to)
	cutoff := passband * 0.5 * math.Min(1, float64(to)/float64(from))
	reach := zeroCrossings / (2 * cutoff)
	half := int(math.Ceil(reach))

	up := to / g
	phases := make([][]float32, up)
	for f := range phases {
		phases[f] = kernel(float64(f)/float64(up), half, cutoff, reach)
	}

	return &resampler{
		up:     int64(up),
		down:   int64(from / g),
		phases: phases,
		held:   make([]float32, half),
		start:  -int64(half),
	}
}

func (r *resampler) feed(in []float32) []float32 {
	r.held = append(r.held, in...)
	end := r.start + int64(len(r.held))
	taps := int64(len(r.phases[0]))

	var out []float32
	for {
		at := r.next * r.down
		first := at/r.up - taps/2 + 1
		if first+taps > end {
			break
		}

		window := r.held[first-r.start:]
		var sum float64
		for j, w := range r.phases[at%r.up] {
			sum += float64(w) * float64(window[j])
		}

		out = append(out, float32(sum))
		r.next++
	}

	used := (r.next*r.down)/r.up - taps/2 + 1 - r.start
	if used > 0 {
		r.held = append(r.held[:0], r.held[used:]...)
		r.start += used
	}

	return out
}

func kernel(frac float64, half int, cutoff, reach float64) []float32 {
	weights := make([]float64, 2*half)
	var sum float64
	for j := range weights {
		d := frac + float64(half-1-j)
		weights[j] = sinc(2*cutoff*d) * blackman(d/reach)
		sum += weights[j]
	}

	out := make([]float32, len(weights))
	for j, w := range weights {
		out[j] = float32(w / sum)
	}

	return out
}

func sinc(x float64) float64 {
	if x == 0 {
		return 1
	}

	return math.Sin(math.Pi*x) / (math.Pi * x)
}

func blackman(x float64) float64 {
	if math.Abs(x) >= 1 {
		return 0
	}

	return 0.42 + 0.5*math.Cos(math.Pi*x) + 0.08*math.Cos(2*math.Pi*x)
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}

	return a
}
