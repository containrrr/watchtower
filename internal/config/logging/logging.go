// Package logging holds console logging settings.
package logging

// Logging holds log format and level settings.
type Logging struct {
	// Level is the maximum log level written to stderr.
	Level string
	// Format is the console log format (auto, logfmt, pretty, json).
	Format string
	// Debug enables debug logging.
	Debug bool
	// Trace enables trace logging.
	Trace bool
	// NoColor disables ANSI color codes.
	NoColor bool
}
