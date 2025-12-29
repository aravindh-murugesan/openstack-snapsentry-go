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
