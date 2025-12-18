package container

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/containrrr/watchtower/internal/util"
	wt "github.com/containrrr/watchtower/pkg/types"

	"github.com/sirupsen/logrus"
)

const (
	namespace        = "com.centurylinklabs.watchtower"
	watchtowerLabel  = namespace
	signalLabel      = namespace + ".stop-signal"
	enableLabel      = namespace + ".enable"
	monitorOnlyLabel = namespace + ".monitor-only"
	noPullLabel      = namespace + ".no-pull"
	dependsOnLabel   = namespace + ".depends-on"
	zodiacLabel      = "com.centurylinklabs.zodiac.original-image"
	scope            = namespace + ".scope"
)

// ContainsWatchtowerLabel takes a map of labels and values and tells
// the consumer whether it contains a valid watchtower instance label
func ContainsWatchtowerLabel(labels map[string]string) bool {
	val, ok := labels[watchtowerLabel]
	return ok && val == "true"
}

// GetLifecycleCommand returns the lifecycle command set in the container metadata or an empty string
func (c *Container) GetLifecycleCommand(phase wt.LifecyclePhase) string {
	label := fmt.Sprintf("%v.lifecycle.%v", namespace, phase)
	value, found := c.getLabelValue(label)

	if !found {
		return ""
	}

	return value
}

// GetLifecycleTimeout checks whether a container has a specific timeout set
// for how long the lifecycle command is allowed to run. This value is expressed
// either as a duration, an integer (minutes implied), or as 0 which will allow the command/script
// to run indefinitely. Users should be cautious with the 0 option, as that
// could result in watchtower waiting forever.
func (c *Container) GetLifecycleTimeout(phase wt.LifecyclePhase) time.Duration {
	label := fmt.Sprintf("%v.lifecycle.%v-timeout", namespace, phase)
	timeout, err := c.getDurationLabelValue(label, time.Minute)

	if err != nil {
		timeout = time.Minute
		if !errors.Is(err, errorLabelNotFound) {
			logrus.WithError(err).Errorf("could not parse timeout label value for %v lifecycle", phase)
		}
	}

	return timeout
}

func (c *Container) getLabelValueOrEmpty(label string) string {
	if val, ok := c.containerInfo.Config.Labels[label]; ok {
		return val
	}
	return ""
}

func (c *Container) getLabelValue(label string) (string, bool) {
	val, ok := c.containerInfo.Config.Labels[label]
	return val, ok
}

func (c *Container) getBoolLabelValue(label string) (bool, error) {
	if strVal, ok := c.containerInfo.Config.Labels[label]; ok {
		value, err := strconv.ParseBool(strVal)
		return value, err
	}
	return false, errorLabelNotFound
}

func (c *Container) getDurationLabelValue(label string, unitlessUnit time.Duration) (time.Duration, error) {
	value, found := c.getLabelValue(label)
	if !found || len(value) < 1 {
		return 0, errorLabelNotFound
	}

	return util.ParseDuration(value, unitlessUnit)
}
