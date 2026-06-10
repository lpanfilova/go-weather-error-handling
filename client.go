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

type Clock interface {
	Now() time.Time
}

func getWeather(clientURL string, sleeper Sleeper, clock Clock) (string, error) {
	for i := range maxAttempts {
		resp, err := http.Get(clientURL)
		if err != nil {
			return "", err
		}

		switch resp.StatusCode {
		case http.StatusOK:
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				return "", err
			}
			resp.Body.Close()
			return string(bodyBytes), nil

		case http.StatusInternalServerError:
			resp.Body.Close()
			return "", &APIError{
				StatusCode: resp.StatusCode,
				StatusText: http.StatusText(resp.StatusCode),
			}

		case http.StatusTooManyRequests:

			if i == maxAttempts-1 {
				resp.Body.Close()
				return "", &APIError{
					StatusCode: resp.StatusCode,
					StatusText: http.StatusText(resp.StatusCode),
				}
			}

			retryHeaderString := resp.Header.Get("Retry-After")
			seconds, err := strconv.Atoi(retryHeaderString)
			if err != nil {
				parsedTime, err := time.Parse(http.TimeFormat, retryHeaderString)
				if err != nil {
					seconds = defaultRetryAfter
				} else {
					seconds = int(parsedTime.Sub(clock.Now()) / time.Second)
					if seconds < 0 {
						seconds = 0
					}
				}
			}

			timeToWait := time.Duration(seconds) * time.Second
			sleeper.Sleep(timeToWait)
			resp.Body.Close()
			continue

		default:
			resp.Body.Close()
			return "", &APIError{
				StatusCode: resp.StatusCode,
				StatusText: http.StatusText(resp.StatusCode),
			}
		}
	}

	return "", fmt.Errorf("retry loop exited unexpectedly after %d attempts", maxAttempts)
}
