package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/maksim-miliutin/Sublayer/internal/app"
	"github.com/maksim-miliutin/Sublayer/internal/capture"
	"github.com/maksim-miliutin/Sublayer/internal/pipeline"
	"github.com/maksim-miliutin/Sublayer/internal/servers"
	"github.com/maksim-miliutin/Sublayer/internal/settings"
	"github.com/maksim-miliutin/Sublayer/internal/ui"
)

func subtitles(args []string, m machine) error {
	asked, s, err := settle(args, m)
	if err != nil {
		return err
	}

	if asked.file != "" {
		return errUsage
	}

	window, err := ui.Open()
	if err != nil {
		return err
	}

	session := app.NewSession(app.Config{Window: window, Shown: s.Pairs, Language: s.Language, Languages: settings.Languages, Target: s.Target})
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

	fmt.Fprintln(os.Stderr, "listening to the speakers and showing subtitles at the bottom left of the screen, each phrase printed here too; press Ctrl+C to stop")
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
