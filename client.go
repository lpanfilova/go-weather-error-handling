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

type requestResult struct {
	weather     string
	shouldRetry bool
	retryDelay  int
	err         error
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
		requestResult := attemptGetWeather(clientURL, clock)
		if requestResult.err != nil {
			return "", requestResult.err
		}

		if requestResult.shouldRetry {
			if i != maxAttempts-1 {
				timeToWait := time.Duration(requestResult.retryDelay) * time.Second
				sleeper.Sleep(timeToWait)
				continue
			} else {
				return "", &APIError{
					StatusCode: http.StatusTooManyRequests,
					StatusText: http.StatusText(http.StatusTooManyRequests),
				}
			}
		}

		return requestResult.weather, nil
	}

	return "", fmt.Errorf("retry loop exited unexpectedly after %d attempts", maxAttempts)
}

func parseRetryHeader(retryHeaderString string, clock Clock) (seconds int) {

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
	if seconds < 0 {
		seconds = 0
	}

	return seconds
}

func attemptGetWeather(clientURL string, clock Clock) *requestResult {

	resp, err := http.Get(clientURL)
	if err != nil {
		return &requestResult{weather: "", shouldRetry: false, retryDelay: 0, err: err}
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return &requestResult{weather: "", shouldRetry: false, retryDelay: 0, err: err}
		}
		return &requestResult{weather: string(bodyBytes), shouldRetry: false, retryDelay: 0, err: nil}

	case http.StatusInternalServerError:
		return &requestResult{weather: "", shouldRetry: false, retryDelay: 0, err: &APIError{
			StatusCode: resp.StatusCode,
			StatusText: http.StatusText(resp.StatusCode),
		}}

	case http.StatusTooManyRequests:
		retryHeaderString := resp.Header.Get("Retry-After")
		seconds := parseRetryHeader(retryHeaderString, clock)
		return &requestResult{weather: "", shouldRetry: true, retryDelay: seconds, err: nil}

	default:
		return &requestResult{weather: "", shouldRetry: false, retryDelay: 0, err: &APIError{
			StatusCode: resp.StatusCode,
			StatusText: http.StatusText(resp.StatusCode),
		}}

	}
}
