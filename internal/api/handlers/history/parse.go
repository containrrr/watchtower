package history

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

// errInvalidTimeParameter is returned when a time parameter cannot be parsed.
var errInvalidTimeParameter = errors.New("invalid time parameter")

// errInvalidLimit is returned when the limit parameter cannot be parsed as an integer.
var errInvalidLimit = errors.New("invalid limit parameter")

// errNoTimeParameter is returned when a time parameter is empty.
var errNoTimeParameter = errors.New("no time parameter provided")

// errNegativeLimit is returned when the limit parameter is negative.
var errNegativeLimit = errors.New("limit must be non-negative")

func parseTimeParam(value string) (*time.Time, error) {
	var noTime *time.Time
	if value == "" {
		return noTime, errNoTimeParameter
	}

	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errInvalidTimeParameter, err)
	}

	return &t, nil
}

func parseLimit(value string) (int, error) {
	if value == "" {
		return 0, nil
	}

	limit, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", errInvalidLimit, err)
	}

	if limit < 0 {
		return 0, errNegativeLimit
	}

	return limit, nil
}
