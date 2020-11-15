package logger

import (
	"context"
	"github.com/sirupsen/logrus"
)

type contextKeyType string

const contextKey = contextKeyType("LogrusLoggerContext")

// GetLogger returns a logger from the context if one is available, otherwise a default logger
func GetLogger(ctx context.Context) *logrus.Logger {
	if logger, ok := ctx.Value(contextKey).(logrus.Logger); ok {
		return &logger
	}
	return newLogger(&logrus.JSONFormatter{}, logrus.InfoLevel)
}

// AddLogger adds a logger to the passed context
func AddLogger(ctx context.Context) context.Context {
	return setLogger(ctx, &logrus.JSONFormatter{}, logrus.InfoLevel)
}

// AddDebugLogger adds a text-formatted debug logger to the passed context
func AddDebugLogger(ctx context.Context) context.Context {
	return setLogger(ctx, &logrus.TextFormatter{}, logrus.DebugLevel)
}

// SetLogger adds a logger to the supplied context
func setLogger(ctx context.Context, fmt logrus.Formatter, level logrus.Level) context.Context {
	log := newLogger(fmt, level)
	return context.WithValue(ctx, contextKey, log)
}

func newLogger(fmt logrus.Formatter, level logrus.Level) *logrus.Logger {
	log := logrus.New()

	log.SetFormatter(fmt)
	log.SetLevel(level)
	return log
}
