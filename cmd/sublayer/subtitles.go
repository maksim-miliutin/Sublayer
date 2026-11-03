package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/maksim-miliutin/Sublayer/internal/app"
	"github.com/maksim-miliutin/Sublayer/internal/capture"
	"github.com/maksim-miliutin/Sublayer/internal/controls"
	"github.com/maksim-miliutin/Sublayer/internal/pipeline"
	"github.com/maksim-miliutin/Sublayer/internal/servers"
	"github.com/maksim-miliutin/Sublayer/internal/settings"
	"github.com/maksim-miliutin/Sublayer/internal/ui"
)

const expireEvery = 250 * time.Millisecond

func subtitles(args []string, m machine) (err error) {
	asked, s, err := settle(args, m)
	if err != nil {
		return err
	}

	if asked.file != "" {
		return errUsage
	}

	keys, err := keysFor(s.Keys)
	if err != nil {
		return err
	}

	panel, err := controls.Open(controls.Config{Languages: settings.Languages, Keys: keys})
	if err != nil {
		return err
	}

	if detach() {
		defer func() {
			if err != nil {
				alarm(err)
			}
		}()
	}

	window, err := ui.Open(ui.Options{Corner: cornerOf(s), Look: lookOf(s)})
	if err != nil {
		return errors.Join(err, panel.Close())
	}

	session := app.NewSession(app.Config{
		Window:    window,
		Panel:     panel,
		Shown:     s.Pairs,
		Language:  s.Language,
		Languages: settings.Languages,
		Target:    s.Target,
		Remember:  func(language string) error { return m.update(speaking(language)) },
		Place:     func(at ui.Point) error { return m.update(placed(at)) },
		MoveHint:  "Drag to move, then " + keyOf(keys, controls.ToggleMove) + " again to fix",
		Linger:    s.Overlay.Linger(),
	})
	ticks := time.NewTicker(expireEvery)
	defer ticks.Stop()
	go expire(session, ticks.C)
	go session.Follow(panel.Choices())
	defer closeOnInterrupt(quitter(session.Quit))()

	stop, err := m.servers(session.Context(), s, func(server servers.Server) {
		session.Starting(server.Name)
		announce(server)
	})
	if err != nil {
		return errors.Join(unlessQuit(err), session.Close())
	}
	defer stop()

	source, err := capture.Open()
	if err != nil {
		return errors.Join(err, session.Close())
	}
	defer source.Close()
	go closeOnQuit(session, source)

	fmt.Fprintln(os.Stderr, "listening to the speakers and showing subtitles at the bottom left of the screen, each phrase printed here too; exit from the tray icon or press Ctrl+C")
	session.Ready()

	v := voicesFor(s)
	err = interpret(pipeline.Config{
		Source:      session.Listen(source),
		Rate:        capture.Rate,
		Waiting:     waitingPhrases,
		Transcriber: session.Transcriber(v.whisper),
		Translator:  session.Translator(v.libre),
		Recognizing: session.Recognizing,
		Report: func(r pipeline.Result) {
			session.Report(r)
			show(r)
		},
	}, s)

	return errors.Join(err, session.Close())
}

func closeOnQuit(session *app.Session, source capture.Source) {
	<-session.Context().Done()
	source.Close()
}

func expire(session *app.Session, ticks <-chan time.Time) {
	for {
		select {
		case <-ticks:
			session.Expire()
		case <-session.Context().Done():
			return
		}
	}
}

func lookOf(s settings.Settings) ui.Look {
	return ui.Look{TextPoints: s.Overlay.TextSize, WidthPercent: s.Overlay.WidthPercent, BackgroundPercent: s.Overlay.BackgroundPercent}
}

func cornerOf(s settings.Settings) *ui.Point {
	if s.Corner == nil {
		return nil
	}

	return &ui.Point{X: s.Corner.X, Y: s.Corner.Y}
}

func speaking(language string) func(settings.Settings) settings.Settings {
	return func(s settings.Settings) settings.Settings {
		s.Language = language

		return s
	}
}

func placed(at ui.Point) func(settings.Settings) settings.Settings {
	return func(s settings.Settings) settings.Settings {
		s.Corner = &settings.Corner{X: at.X, Y: at.Y}

		return s
	}
}
