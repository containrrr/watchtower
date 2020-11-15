package logger

import (
	"context"
	"github.com/sirupsen/logrus"
)

const ContextKey = "LogrusLoggerContext"

// GetLogger returns a logger from the context if one is available, otherwise a default logger
func GetLogger(ctx context.Context) *logrus.Logger {
	if logger, ok := ctx.Value(ContextKey).(logrus.Logger); ok {
		return &logger
	} else {
		return newLogger(&logrus.JSONFormatter{}, logrus.InfoLevel)
	}
}

func AddLogger(ctx context.Context) {
	setLogger(ctx, &logrus.JSONFormatter{}, logrus.InfoLevel)
}

func AddDebugLogger(ctx context.Context) {
	setLogger(ctx, &logrus.TextFormatter{}, logrus.DebugLevel)
}

// SetLogger adds a logger to the supplied context
func setLogger(ctx context.Context, fmt logrus.Formatter, level logrus.Level) {
	log := newLogger(fmt, level)
	context.WithValue(ctx, ContextKey, log)
}

func newLogger(fmt logrus.Formatter, level logrus.Level) *logrus.Logger {
	log := logrus.New()

	log.SetFormatter(fmt)
	log.SetLevel(level)
	return log
}