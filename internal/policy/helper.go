package policy

import (
	"fmt"
	"time"

	"github.com/go-viper/mapstructure/v2"
)

// ParseSnapSentryMetadataFromSDK is a generic helper to unmarshal a map[string]string
// into a strongly-typed policy struct using JSON tags.
// It uses weak typing to handle string-to-int/bool conversions.
func ParseSnapSentryMetadataFromSDK[T any](metadata map[string]string) (*T, error) {
	var result T

	config := &mapstructure.DecoderConfig{
		Result:           &result,
		WeaklyTypedInput: true,
		TagName:          "json",
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeHookFunc(time.RFC3339),
		),
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return nil, err
	}

	if err := decoder.Decode(metadata); err != nil {
		return nil, err
	}

	return &result, nil
}

// helperNormalizeTimezone loads a Time Location from a string name.
// It defaults to UTC if the timezone string is empty.
func helperNormalizeTimezone(timezone string) (string, *time.Location, error) {
	if timezone == "" {
		timezone = "UTC"
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return timezone, nil, fmt.Errorf("invalid timezone '%s': %w", timezone, err)
	}
	return timezone, loc, nil
}

// helperNormalizeRetentionDays ensures the retention period is valid.
// If the provided days are <= 0, it falls back to the specified default.
func helperNormalizeRetentionDays(days int, defaultDays int) int {
	if days <= 0 {
		return defaultDays
	}
	return days
}

// helperNormalizeStartTime parses a time string in "HH:MM" or "HH:MM:SS" format.
// It defaults to "00:00:00" if the input is empty.
func helperNormalizeStartTime(startTime string) (time.Time, error) {
	if startTime == "" {
		startTime = "00:00:00"
	}

	// Try parsing short format (HH:MM)
	t, err := time.Parse("15:04", startTime)
	if err == nil {
		return t, nil
	}

	// Try parsing long format (HH:MM:SS)
	t, err = time.Parse(time.TimeOnly, startTime)
	if err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("invalid start time '%s'; must be HH:MM or HH:MM:SS", startTime)
}

// helperNormalizeDay converts various string representations of a weekday into a time.Weekday.
// It supports full names ("Monday"), short names ("Mon"), and numeric strings ("1").
func helperNormalizeDay(dayStr string) (time.Weekday, error) {

	if dayStr == "" {
		dayStr = "sunday"
	}

	switch dayStr {
	case "Sunday", "Sun", "sun", "sunday", "0":
		return time.Sunday, nil
	case "Monday", "Mon", "mon", "monday", "1":
		return time.Monday, nil
	case "Tuesday", "Tue", "tue", "tuesday", "2":
		return time.Tuesday, nil
	case "Wednesday", "Wed", "wed", "wednesday", "3":
		return time.Wednesday, nil
	case "Thursday", "Thu", "thu", "thursday", "4":
		return time.Thursday, nil
	case "Friday", "Fri", "fri", "friday", "5":
		return time.Friday, nil
	case "Saturday", "Sat", "sat", "saturday", "6":
		return time.Saturday, nil
	default:
		return 0, fmt.Errorf("invalid day '%s'", dayStr)
	}
}
