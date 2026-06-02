// client.go

package main

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

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
		seconds, err := strconv.Atoi(resp.Header.Get("Retry-After"))
		if err != nil {
			return "", err
		}
		timeToWait := time.Duration(seconds) * time.Second
		sleeper.Sleep(timeToWait)
		return getWeather(clientURL, sleeper)

	default:
		return "", &APIError{
			StatusCode: resp.StatusCode,
			StatusText: http.StatusText(resp.StatusCode),
		}
	}
}
