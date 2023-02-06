// Package cmd contains the watchtower (sub-)commands
package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/containrrr/watchtower/internal/flags"
	"github.com/containrrr/watchtower/pkg/container"
	"github.com/containrrr/watchtower/pkg/notifications"
	"github.com/spf13/cobra"
)

var notifyUpgradeCommand = NewNotifyUpgradeCommand()

// NewNotifyUpgradeCommand creates the notify upgrade command for watchtower
func NewNotifyUpgradeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "notify-upgrade",
		Short: "Upgrade legacy notification configuration to shoutrrr URLs",
		Run:   runNotifyUpgrade,
	}
}

func runNotifyUpgrade(cmd *cobra.Command, args []string) {
	if err := runNotifyUpgradeE(cmd, args); err != nil {
		logf("Notification upgrade failed: %v", err)
	}
}

func runNotifyUpgradeE(cmd *cobra.Command, _ []string) error {
	f := cmd.Flags()
	flags.ProcessFlagAliases(f)

	notifier = notifications.NewNotifier(cmd)
	urls := notifier.GetURLs()

	logf("Found notification configurations for: %v", strings.Join(notifier.GetNames(), ", "))

	outFile, err := os.CreateTemp("/", "watchtower-notif-urls-*")
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	logf("Writing notification URLs to %v", outFile.Name())
	logf("")

	sb := strings.Builder{}
	sb.WriteString("WATCHTOWER_NOTIFICATION_URL=")

	for i, u := range urls {
		if i != 0 {
			sb.WriteRune(' ')
		}
		sb.WriteString(u)
	}

	_, err = fmt.Fprint(outFile, sb.String())
	tryOrLog(err, "Failed to write to output file")

	tryOrLog(outFile.Sync(), "Failed to sync output file")
	tryOrLog(outFile.Close(), "Failed to close output file")

	containerID := "<CONTAINER>"
	cid, err := container.GetRunningContainerID()
	tryOrLog(err, "Failed to get running container ID")
	if cid != "" {
		containerID = cid.ShortID()
	}
	logf("To get the environment file, use:")
	logf("cp %v:%v ./watchtower-notifications.env", containerID, outFile.Name())
	logf("")
	logf("Note: This file will be removed in 5 minutes or when this container is stopped!")

	signalChannel := make(chan os.Signal, 1)
	time.AfterFunc(5*time.Minute, func() {
		signalChannel <- syscall.SIGALRM
	})

	signal.Notify(signalChannel, os.Interrupt)
	signal.Notify(signalChannel, syscall.SIGTERM)

	switch <-signalChannel {
	case syscall.SIGALRM:
		logf("Timed out!")
	case os.Interrupt, syscall.SIGTERM:
		logf("Stopping...")
	default:
	}

	if err := os.Remove(outFile.Name()); err != nil {
		logf("Failed to remove file, it may still be present in the container image! Error: %v", err)
	} else {
		logf("Environment file has been removed.")
	}

	return nil
}

func tryOrLog(err error, message string) {
	if err != nil {
		logf("%v: %v\n", message, err)
	}
}

func logf(format string, v ...interface{}) {
	fmt.Fprintln(os.Stderr, fmt.Sprintf(format, v...))
}
