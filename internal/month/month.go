package month

import (
	"fmt"
	"time"
)

const layout = "01-2006"

type Month time.Time

func Parse(value string) (time.Time, error) {
	parsed, err := time.Parse(layout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid month-year value %q: %w", value, err)
	}

	return time.Date(parsed.Year(), parsed.Month(), 1, 0, 0, 0, 0, time.UTC), nil
}

func MustParse(value string) time.Time {
	parsed, err := Parse(value)
	if err != nil {
		panic(err)
	}

	return parsed
}

func Format(value time.Time) string {
	return value.UTC().Format(layout)
}

func LastDay(value time.Time) time.Time {
	monthStart := time.Date(value.Year(), value.Month(), 1, 0, 0, 0, 0, time.UTC)
	return monthStart.AddDate(0, 1, -1)
}
