// client_test.go

package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type responseConfig struct {
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

type respSequenceServer struct {
	respSeq  []int
	reqCount int
	config   responseConfig
}

func (s *respSequenceServer) Inc() {
	s.reqCount++
}

func TestGetWeather(t *testing.T) {
	t.Run("returns weather when server responds 200 OK", func(t *testing.T) {
		sleeper := &SpySleeper{}

		serverState := &respSequenceServer{
			respSeq:  []int{http.StatusOK},
			reqCount: 0,
			config: responseConfig{
				responseBody:      "sunny",
				retryAfter:        "",
				includeRetryAfter: false,
			},
		}
		server := makeConfigurableServer(serverState)
		defer server.Close()

		want := "sunny"
		got, err := getWeather(server.URL, sleeper)
		if err != nil {
			t.Fatalf("failed to call server: %v", err)
		}

		assertWeatherString(t, got, want)
		assertNumRequests(t, serverState.reqCount, 1)
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
				serverState := &respSequenceServer{
					respSeq:  []int{tt.serverResponse},
					reqCount: 0,
					config: responseConfig{
						responseBody:      tt.weather,
						retryAfter:        "",
						includeRetryAfter: false,
					},
				}
				server := makeConfigurableServer(serverState)
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
			config           responseConfig
		}{
			{
				name:             "uses Retry-After when valid",
				weather:          "rainy",
				wantSleepSeconds: 4,
				config: responseConfig{
					responseBody:      "rainy",
					retryAfter:        "4",
					includeRetryAfter: true},
			},
			{
				name:             "uses default delay when Retry-After is invalid",
				weather:          "rainy",
				wantSleepSeconds: 2,
				config: responseConfig{
					responseBody:      "rainy",
					retryAfter:        "invalid",
					includeRetryAfter: true},
			},
			{
				name:             "uses default delay when Retry-After is empty",
				weather:          "rainy",
				wantSleepSeconds: 2,
				config: responseConfig{
					responseBody:      "rainy",
					retryAfter:        "",
					includeRetryAfter: true},
			},
			{
				name:             "uses default delay when Retry-After is absent",
				weather:          "rainy",
				wantSleepSeconds: 2,
				config: responseConfig{
					responseBody:      "rainy",
					retryAfter:        "",
					includeRetryAfter: false},
			},
		}

		for _, tt := range retryTests {
			t.Run(tt.name, func(t *testing.T) {
				requiredSleep := time.Duration(tt.wantSleepSeconds) * time.Second
				sleeper := &SpySleeper{}

				serverState := &respSequenceServer{
					respSeq:  []int{429, 200},
					reqCount: 0,
					config:   tt.config,
				}
				server := makeConfigurableServer(serverState)
				defer server.Close()

				want := tt.weather
				got, err := getWeather(server.URL, sleeper)
				if err != nil {
					t.Fatalf("client failed to retry: %v", err)
				}

				assertTimeSlept(t, sleeper.lastSleepDuration, requiredSleep)
				assertWeatherString(t, got, want)
			})
		}
	})

	t.Run("handles infinite 429 retry responses", func(t *testing.T) {
		totalRequiredSleep := time.Duration(2*maxRetryCount) * time.Second
		sleeper := &SpySleeper{}

		serverState := &respSequenceServer{
			respSeq:  []int{429, 429, 429, 429, 429},
			reqCount: 0,
			config: responseConfig{
				responseBody:      "",
				retryAfter:        "2",
				includeRetryAfter: true,
			},
		}
		server := makeConfigurableServer(serverState)
		defer server.Close()

		want := ""
		got, err := getWeather(server.URL, sleeper)
		if err == nil {
			t.Fatal("expected an error, got nil")
		}

		apiErr := requireAPIError(t, err)
		assertStatusCode(t, apiErr.StatusCode, http.StatusTooManyRequests)
		assertNumRequests(t, serverState.reqCount, 4)
		assertTimeSlept(t, sleeper.totalTimeSlept, totalRequiredSleep)
		assertWeatherString(t, got, want)
	})
}

func makeConfigurableServer(s *respSequenceServer) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.config.includeRetryAfter {
			w.Header().Set("Retry-After", s.config.retryAfter)
		}
		w.WriteHeader(s.respSeq[s.reqCount])
		w.Write([]byte(s.config.responseBody))
		s.Inc()
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

func assertTimeSlept(t testing.TB, got, want time.Duration) {
	t.Helper()
	if got != want {
		t.Errorf("slept for %v, want to sleep for %v", got, want)
	}
}

func assertNumRequests(t testing.TB, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("called server for %d times, want to call for %d", got, want)
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
