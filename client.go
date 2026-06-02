// client.go

package main

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const defaultRetryAfter = 2
const maxRetryCount = 3

const maxAttempts = maxRetryCount + 1

type APIError struct {
	StatusCode int
	StatusText string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("api returned status %d: %s", e.StatusCode, e.StatusText)
}

type Sleeper interface {
	Sleep(time.Duration)
}

type DefaultSleeper struct{}

func (*DefaultSleeper) Sleep(duration time.Duration) {
	time.Sleep(duration)
}

func getWeather(clientURL string, sleeper Sleeper) (string, error) {
	for i := range maxAttempts {
		resp, err := http.Get(clientURL)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusOK:
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				return "", err
			}
			return string(bodyBytes), nil

		case http.StatusInternalServerError:
			return "", &APIError{
				StatusCode: resp.StatusCode,
				StatusText: http.StatusText(resp.StatusCode),
			}

		case http.StatusTooManyRequests:

			if i == maxAttempts-1 {
				return "", &APIError{
					StatusCode: resp.StatusCode,
					StatusText: http.StatusText(resp.StatusCode),
				}
			}

			seconds, err := strconv.Atoi(resp.Header.Get("Retry-After"))
			if err != nil {
				seconds = defaultRetryAfter
			}
			timeToWait := time.Duration(seconds) * time.Second
			sleeper.Sleep(timeToWait)
			resp.Body.Close()
			continue

		default:
			return "", &APIError{
				StatusCode: resp.StatusCode,
				StatusText: http.StatusText(resp.StatusCode),
			}
		}
	}

	return "", fmt.Errorf("retry loop exited unexpectedly after %d attempts", maxAttempts)
}
