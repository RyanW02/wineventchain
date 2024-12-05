package types

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"
)

type MarshalledDuration time.Duration

var durationRegex = regexp.MustCompile(`^(?:(\d+)w)? ?(?:(\d+)d)? ?(?:(\d+)h)? ?(?:(\d+)m)? ?(?:(\d+)s)? ?(?:(\d+)ms)? ?(?:(\d+)ns)?$`)

func (d MarshalledDuration) Duration() time.Duration {
	return time.Duration(d)
}

func (d MarshalledDuration) MarshalJSON() ([]byte, error) {
	return []byte(`"` + time.Duration(d).String() + `"`), nil
}

func (d *MarshalledDuration) UnmarshalJSON(data []byte) error {
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return errors.New("invalid duration: missing quotes")
	}

	duration, err := parseDuration(string(data[1 : len(data)-1]))
	if err != nil {
		return err
	}

	*d = MarshalledDuration(duration)
	return nil
}

func (d *MarshalledDuration) UnmarshalText(text []byte) error {
	duration, err := parseDuration(string(text))
	if err != nil {
		return err
	}

	*d = MarshalledDuration(duration)
	return nil
}

func (d *MarshalledDuration) UnmarshalYAML(data []byte) error {
	return d.UnmarshalText(data)
}

// parseDuration: Go's native time.ParseDuration function does not support days or weeks.
func parseDuration(s string) (time.Duration, error) {
	if s == "0" {
		return 0, nil
	}

	groups := durationRegex.FindStringSubmatch(s)
	if len(groups) != 8 {
		return 0, fmt.Errorf("invalid duration: %s", s)
	}

	var duration time.Duration

	// Weeks
	if groups[1] != "" {
		weeks, err := strconv.Atoi(groups[1])
		if err != nil {
			return 0, err
		}

		duration += time.Duration(weeks) * 7 * 24 * time.Hour
	}

	// Days
	if groups[2] != "" {
		days, err := strconv.Atoi(groups[2])
		if err != nil {
			return 0, err
		}

		duration += time.Duration(days) * 24 * time.Hour
	}

	// Hours
	if groups[3] != "" {
		hours, err := strconv.Atoi(groups[3])
		if err != nil {
			return 0, err
		}

		duration += time.Duration(hours) * time.Hour
	}

	// Minutes
	if groups[4] != "" {
		minutes, err := strconv.Atoi(groups[4])
		if err != nil {
			return 0, err
		}

		duration += time.Duration(minutes) * time.Minute
	}

	// Seconds
	if groups[5] != "" {
		seconds, err := strconv.Atoi(groups[5])
		if err != nil {
			return 0, err
		}

		duration += time.Duration(seconds) * time.Second
	}

	// Milliseconds
	if groups[6] != "" {
		milliseconds, err := strconv.Atoi(groups[6])
		if err != nil {
			return 0, err
		}

		duration += time.Duration(milliseconds) * time.Millisecond
	}

	// Nanoseconds
	if groups[7] != "" {
		nanoseconds, err := strconv.Atoi(groups[7])
		if err != nil {
			return 0, err
		}

		duration += time.Duration(nanoseconds)
	}

	return duration, nil
}
