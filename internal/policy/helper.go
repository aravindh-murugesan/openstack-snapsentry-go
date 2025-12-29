package policy

import (
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
