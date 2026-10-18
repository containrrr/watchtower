// Package cmd defines Watchtower's command-line interface.
//
// The root command loads the configuration, connects to Docker, and updates the
// watched containers once, on a schedule, or alongside the HTTP API. The
// notify-upgrade command converts legacy notification settings to Shoutrrr
// URLs.
package cmd
