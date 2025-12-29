package cloud

import "time"

// RetryConfig defines the parameters for the exponential backoff and retry mechanism.
// It allows fine-tuning of how aggressive the system should be when handling transient errors.
type RetryConfig struct {
	// MaxRetries is the maximum number of additional attempts after the initial failure.
	// For example, if MaxRetries is 3, the operation runs at most 4 times (1 initial + 3 retries).
	MaxRetries int

	// BaseDelay is the initial wait time before the first retry.
	// This duration increases exponentially with each attempt (BaseDelay * 2^attempt).
	BaseDelay time.Duration

	// MaxDelay is the hard limit for the sleep duration between retries.
	// Even if the exponential calculation exceeds this value, the wait time will be capped here.
	MaxDelay time.Duration

	// OperationTimeout is the total time limit for the entire operation, including all retries.
	// If this timeout is reached, the context will be cancelled regardless of retry attempts left.
	OperationTimeout time.Duration
}
