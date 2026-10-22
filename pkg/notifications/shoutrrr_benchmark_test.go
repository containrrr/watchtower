package notifications

import (
	"io"
	"testing"

	"github.com/rs/zerolog"
)

type eventFieldMapHook struct{}

func (eventFieldMapHook) Run(event *zerolog.Event, _ zerolog.Level, _ string) {
	fields, _, extracted := eventFieldMap(event)
	if !extracted || fields == nil {
		return
	}
}

func BenchmarkEventFieldMap(b *testing.B) {
	log := zerolog.New(io.Discard).Hook(eventFieldMapHook{})

	b.ReportAllocs()

	for b.Loop() {
		log.Info().
			Str("container", "web").
			Str("image", "app:latest").
			Msg("Found new image")
	}
}
