package rag

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type HTTPClientOptions struct {
	Timeout      time.Duration
	MaxRetries   int
	RetryBackoff time.Duration
}

func normalizeHTTPClientOptions(options HTTPClientOptions, defaultTimeout time.Duration) HTTPClientOptions {
	if options.Timeout <= 0 {
		options.Timeout = defaultTimeout
	}
	if options.MaxRetries < 0 {
		options.MaxRetries = 0
	}
	if options.RetryBackoff <= 0 {
		options.RetryBackoff = 250 * time.Millisecond
	}
	return options
}

// DoWithRetry retries transient transport errors and 429/502/503/504 responses.
// newRequest must return a fresh request body for every attempt.
func DoWithRetry(
	ctx context.Context,
	client *http.Client,
	backend string,
	options HTTPClientOptions,
	newRequest func() (*http.Request, error),
) (*http.Response, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if options.MaxRetries < 0 {
		options.MaxRetries = 0
	}
	if options.RetryBackoff <= 0 {
		options.RetryBackoff = 250 * time.Millisecond
	}
	for attempt := 0; ; attempt++ {
		request, err := newRequest()
		if err != nil {
			return nil, err
		}
		response, requestErr := client.Do(request)
		if !shouldRetryHTTP(response, requestErr) || attempt >= options.MaxRetries {
			return response, requestErr
		}
		if response != nil {
			response.Body.Close()
		}
		observeRAGRetry(backend)
		delay := retryDelay(options.RetryBackoff, attempt, response)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func shouldRetryHTTP(response *http.Response, err error) bool {
	if err != nil {
		return true
	}
	if response == nil {
		return false
	}
	switch response.StatusCode {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func retryDelay(base time.Duration, attempt int, response *http.Response) time.Duration {
	if response != nil {
		if value := strings.TrimSpace(response.Header.Get("Retry-After")); value != "" {
			if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
				return time.Duration(seconds) * time.Second
			}
			if when, err := http.ParseTime(value); err == nil {
				if delay := time.Until(when); delay > 0 {
					return delay
				}
			}
		}
	}
	if attempt > 5 {
		attempt = 5
	}
	return base * time.Duration(1<<attempt)
}

func requestCreationError(backend string, err error) error {
	return fmt.Errorf("could not create %s request: %w", backend, err)
}
