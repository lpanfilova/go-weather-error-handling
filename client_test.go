// client_test.go

package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type retryServerConfig struct {
	responseBody      string
	retryAfter        string
	includeRetryAfter bool
}

type SpySleeper struct {
	lastSleepDuration time.Duration
	totalTimeSlept    time.Duration
}

func (s *SpySleeper) Sleep(duration time.Duration) {
	s.lastSleepDuration = duration
	s.totalTimeSlept += duration
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

	t.Run("handles non-retryable errors", func(t *testing.T) {
		errorTests := []struct {
			name           string
			weather        string
			want           string
			serverResponse int
		}{
			{
				name:           "returns error when server responds 500",
				weather:        "sunny",
				want:           "",
				serverResponse: http.StatusInternalServerError,
			},
			{
				name:           "returns API error when server responds with unexpected status",
				weather:        "sunny",
				want:           "",
				serverResponse: 530,
			},
		}

		for _, tt := range errorTests {
			t.Run(tt.name, func(t *testing.T) {
				sleeper := &SpySleeper{}
				server := makeServer(tt.weather, tt.serverResponse)
				defer server.Close()

				got, err := getWeather(server.URL, sleeper)
				if err == nil {
					t.Fatal("expected an error, got nil")
				}

				apiErr := requireAPIError(t, err)
				assertWeatherString(t, got, tt.want)
				assertStatusCode(t, apiErr.StatusCode, tt.serverResponse)
			})
		}
	})

	t.Run("handles 429 retry responses", func(t *testing.T) {
		retryTests := []struct {
			name             string
			weather          string
			wantSleepSeconds int
			config           retryServerConfig
		}{
			{
				name:             "uses Retry-After when valid",
				weather:          "rainy",
				wantSleepSeconds: 4,
				config: retryServerConfig{
					responseBody:      "rainy",
					retryAfter:        "4",
					includeRetryAfter: true},
			},
			{
				name:             "uses default delay when Retry-After is invalid",
				weather:          "rainy",
				wantSleepSeconds: 2,
				config: retryServerConfig{
					responseBody:      "rainy",
					retryAfter:        "invalid",
					includeRetryAfter: true},
			},
			{
				name:             "uses default delay when Retry-After is empty",
				weather:          "rainy",
				wantSleepSeconds: 2,
				config: retryServerConfig{
					responseBody:      "rainy",
					retryAfter:        "",
					includeRetryAfter: true},
			},
			{
				name:             "uses default delay when Retry-After is absent",
				weather:          "rainy",
				wantSleepSeconds: 2,
				config: retryServerConfig{
					responseBody:      "rainy",
					retryAfter:        "",
					includeRetryAfter: false},
			},
		}

		for _, tt := range retryTests {
			t.Run(tt.name, func(t *testing.T) {
				requiredSleep := time.Duration(tt.wantSleepSeconds) * time.Second
				sleeper := &SpySleeper{}

				server := makeRetryServer(tt.config)
				defer server.Close()

				want := tt.weather
				got, err := getWeather(server.URL, sleeper)
				if err != nil {
					t.Fatalf("client failed to retry: %v", err)
				}

				assertTime(t, sleeper.lastSleepDuration, requiredSleep)
				assertWeatherString(t, got, want)
			})
		}
	})
}

func makeServer(body string, status int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
}

func makeRetryServer(config retryServerConfig) *httptest.Server {
	requestCount := 0

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		if requestCount == 1 {
			if config.includeRetryAfter {
				w.Header().Set("Retry-After", config.retryAfter)
			}
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(config.responseBody))
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
		t.Errorf("got status code %d, want %d", got, want)
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
