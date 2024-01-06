package util

import (
	"strconv"
	"time"
)

// ParseDuration parses the input string as a duration, treating a plain number as implicitly using the specified unit
func ParseDuration(input string, unitlessUnit time.Duration) (time.Duration, error) {
	if unitless, err := strconv.Atoi(input); err == nil {
		return unitlessUnit * time.Duration(unitless), nil
	}

	return time.ParseDuration(input)
}
