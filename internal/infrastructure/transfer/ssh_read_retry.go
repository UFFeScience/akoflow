package transfer

import (
	"context"
	"strings"
	"time"
)

// Only read-only probes may use this helper; retrying activity commands could
// execute user work twice after an ambiguous disconnect.
func retrySSHRead(ctx context.Context, delay time.Duration, read func() ([]byte, error)) ([]byte, error) {
	for attempt := 0; ; attempt++ {
		output, err := read()
		message := strings.ToLower(string(output))
		transient := strings.Contains(message, "connection closed") || strings.Contains(message, "connection reset") || strings.Contains(message, "broken pipe")
		if err == nil || !transient || attempt == 2 || ctx.Err() != nil {
			return output, err
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return output, ctx.Err()
		case <-timer.C:
		}
	}
}
