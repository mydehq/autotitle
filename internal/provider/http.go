package provider

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/mydehq/autotitle/internal/types"
)

// DoWithRetry executes an HTTP request with exponential backoff for 429 errors.
func DoWithRetry(ctx context.Context, client *http.Client, req *http.Request, service string, preRequest func()) (*http.Response, error) {
	const maxRetries = 3
	for i := 0; i <= maxRetries; i++ {
		if preRequest != nil {
			preRequest()
		}
		resp, err := client.Do(req.WithContext(ctx))
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			_ = resp.Body.Close()
			if i == maxRetries {
				return nil, types.ErrAPIError{
					Service:    service,
					StatusCode: 429,
					Message:    "rate limit exceeded after retries",
				}
			}

			// Default wait 2s, or respect Retry-After
			wait := 2 * time.Second
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				if seconds, err := strconv.Atoi(retryAfter); err == nil {
					wait = time.Duration(seconds) * time.Second
				}
			}

			// Exponential backoff: 2s, 4s, 8s...
			time.Sleep(wait * time.Duration(1<<i))
			continue
		}

		return resp, nil
	}
	return nil, fmt.Errorf("request failed after retries")
}
