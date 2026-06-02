// client_test.go

package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

type SpySleeper struct {
	sleepDuration time.Duration
}

func (s *SpySleeper) Sleep(duration time.Duration) {
	s.sleepDuration = duration
}

func TestGetWeather(t *testing.T) {
	t.Run("returns weather when server responds 200 OK", func(t *testing.T) {
		sleeper := &SpySleeper{}
		server := makeServer("sunny", http.StatusOK)

		defer server.Close()

		want := "sunny"
		got, err := getWeather(server.URL, sleeper)
		if err != nil {
			t.Fatalf("failed to call server: %v", err)
		}

		assertWeatherString(t, got, want)
	})

	t.Run("returns error when server responds 500", func(t *testing.T) {
		sleeper := &SpySleeper{}
		server := makeServer("sunny", http.StatusInternalServerError)
		defer server.Close()

		want := ""
		got, err := getWeather(server.URL, sleeper)
		if err == nil {
			t.Fatal("expected an error, got nil")
		}

		apiErr := requireAPIError(t, err)

		assertWeatherString(t, got, want)
		assertStatusCode(t, apiErr.StatusCode, http.StatusInternalServerError)
	})

	t.Run("waits for Retry-After and retries after 429", func(t *testing.T) {
		timeToSleep := 2
		server := makeRetryServer("sunny", timeToSleep)
		defer server.Close()

		sleeper := &SpySleeper{}

		want := "sunny"
		got, err := getWeather(server.URL, sleeper)
		if err != nil {
			t.Fatalf("server failed to retry: %v", err)
		}

		assertWeatherString(t, got, want)

		requiredSleep := time.Duration(timeToSleep) * time.Second
		assertTime(t, sleeper.sleepDuration, requiredSleep)
	})
}

func makeServer(body string, status int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
}

func makeRetryServer(body string, retrySec int) *httptest.Server {
	requestCount := 0

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		if requestCount == 1 {
			w.Header().Set("Retry-After", strconv.Itoa(retrySec))
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	}))
}

func assertWeatherString(t testing.TB, got, want string) {
	t.Helper()

	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func assertStatusCode(t testing.TB, got, want int) {
	t.Helper()

	if got != want {
		t.Errorf("got error %d want %d", got, want)
	}
}

func assertTime(t testing.TB, got, want time.Duration) {
	t.Helper()

	if got != want {
		t.Errorf("slept for %v, want to sleep for %v", got, want)
	}
}

func requireAPIError(t testing.TB, err error) *APIError {
	t.Helper()
	var apiErr *APIError

	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}

	return apiErr
}
