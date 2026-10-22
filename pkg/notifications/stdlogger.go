package notifications

import (
	"io"
	"log"
)

// newStdLogger returns a standard library logger writing to writer, as required by the Shoutrrr sender.
//
// Parameters:
//   - writer: Destination for log output.
//   - prefix: Prefix prepended to each log line.
//
// Returns:
//   - *log.Logger: Logger with no flags set.
func newStdLogger(writer io.Writer, prefix string) *log.Logger {
	return log.New(writer, prefix, 0)
}
