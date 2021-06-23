package types

import "github.com/spf13/cobra"

// ConvertibleNotifier is a notifier capable of creating a shoutrrr URL
type ConvertibleNotifier interface {
	GetURL(c *cobra.Command) (string, error)
}
