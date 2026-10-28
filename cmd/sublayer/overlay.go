package main

import (
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/maksim-miliutin/Sublayer/internal/ui"
)

const (
	samplePause = 2 * time.Second
	shownPairs  = 4
)

var samples = []ui.Pair{
	{Original: "Bonjour, merci d'être là aujourd'hui.", Translation: "Здравствуйте, спасибо, что пришли сегодня."},
	{Original: "Nous allons commencer par votre contrat de travail.", Translation: "Начнём с вашего трудового договора."},
	{Original: "Avez-vous des questions sur l'article trois, celui de la période d'essai ?", Translation: "У вас есть вопросы по третьей статье, той, что об испытательном сроке?"},
	{Original: "The deadline is next Friday.", Translation: "Срок — следующая пятница."},
	{Original: "Please send me the signed copy by email.", Translation: "Пожалуйста, пришлите мне подписанный экземпляр по почте."},
	{Original: "Je vous rappelle demain matin.", Translation: "Я перезвоню вам завтра утром."},
}

var sampleStates = []ui.State{ui.Listening, ui.Recognizing, ui.Listening, ui.Paused, ui.Listening, ui.Failed}

func overlay() error {
	window, err := ui.Open()
	if err != nil {
		return err
	}
	defer window.Close()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	defer signal.Stop(interrupt)

	fmt.Fprintln(os.Stderr, "showing sample subtitles at the bottom left; press Ctrl+C to stop")

	ticker := time.NewTicker(samplePause)
	defer ticker.Stop()

	for n := 0; ; n++ {
		if err := window.Show(sampleFrame(n)); err != nil {
			return err
		}

		select {
		case <-interrupt:
			return window.Close()
		case <-ticker.C:
		}
	}
}

func sampleFrame(n int) ui.Frame {
	var pairs []ui.Pair
	for i := max(0, n-shownPairs+1); i <= n; i++ {
		pairs = append(pairs, samples[i%len(samples)])
	}

	return ui.Frame{Pairs: pairs, State: sampleStates[n%len(sampleStates)]}
}
