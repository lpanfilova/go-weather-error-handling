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
	statusCode        int
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

type FixedClock struct {
	now time.Time
}

func (c *FixedClock) Now() time.Time {
	return c.now
}

type respSequenceServer struct {
	statuses     []responseConfig
	requestCount int
}

func TestGetWeather(t *testing.T) {
	t.Run("returns weather when server responds 200 OK", func(t *testing.T) {
		sleeper := &SpySleeper{}
		clock := &FixedClock{now: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)}

		serverState := &respSequenceServer{
			statuses: []responseConfig{{
				statusCode:   http.StatusOK,
				responseBody: "sunny",
			}},
			requestCount: 0,
		}
		server := makeConfigurableServer(t, serverState)
		defer server.Close()

		want := "sunny"
		got, err := getWeather(server.URL, sleeper, clock)
		if err != nil {
			t.Fatalf("failed to call server: %v", err)
		}

		assertWeatherString(t, got, want)
		assertNumRequests(t, serverState.requestCount, 1)
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
				clock := &FixedClock{now: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)}
				serverState := &respSequenceServer{
					statuses: []responseConfig{{
						statusCode:   tt.serverResponse,
						responseBody: tt.weather,
					}},
					requestCount: 0,
				}
				server := makeConfigurableServer(t, serverState)
				defer server.Close()

				got, err := getWeather(server.URL, sleeper, clock)
				if err == nil {
					t.Fatal("expected an error, got nil")
				}

				apiErr := requireAPIError(t, err)
				assertWeatherString(t, got, tt.want)
				assertStatusCode(t, apiErr.StatusCode, tt.serverResponse)
				assertNumRequests(t, serverState.requestCount, 1)
			})
		}
	})

	t.Run("handles 429 retry responses", func(t *testing.T) {
		clock := &FixedClock{now: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)}

		retryTests := []struct {
			name              string
			retryAfter        string
			includeRetryAfter bool
			wantSleepSeconds  int
		}{
			{
				name:              "uses Retry-After when valid",
				retryAfter:        "4",
				includeRetryAfter: true,
				wantSleepSeconds:  4,
			},
			{
				name:              "uses default delay when Retry-After is invalid",
				retryAfter:        "invalid",
				includeRetryAfter: true,
				wantSleepSeconds:  2,
			},
			{
				name:              "uses default delay when Retry-After is empty",
				retryAfter:        "",
				includeRetryAfter: true,
				wantSleepSeconds:  2,
			},
			{
				name:              "uses default delay when Retry-After is absent",
				includeRetryAfter: false,
				wantSleepSeconds:  2,
			},
			{
				name:              "parses Retry-After when field is in DateTime format",
				retryAfter:        clock.Now().UTC().Add(time.Duration(7) * time.Second).Format(http.TimeFormat),
				includeRetryAfter: true,
				wantSleepSeconds:  7,
			},
		}

		for _, tt := range retryTests {
			t.Run(tt.name, func(t *testing.T) {
				requiredSleep := time.Duration(tt.wantSleepSeconds) * time.Second
				sleeper := &SpySleeper{}

				responses := []responseConfig{
					{
						statusCode:        http.StatusTooManyRequests,
						retryAfter:        tt.retryAfter,
						includeRetryAfter: tt.includeRetryAfter,
					},
					{
						statusCode:   http.StatusOK,
						responseBody: "rainy",
					},
				}

				serverState := &respSequenceServer{
					statuses:     responses,
					requestCount: 0,
				}

				server := makeConfigurableServer(t, serverState)
				defer server.Close()

				_, err := getWeather(server.URL, sleeper, clock)
				if err != nil {
					t.Fatalf("client failed to retry: %v", err)
				}

				assertTimeSlept(t, sleeper.lastSleepDuration, requiredSleep)
				assertNumRequests(t, serverState.requestCount, 2)
			})
		}
	})

	t.Run("stops after max retries with 429 response", func(t *testing.T) {
		totalRequiredSleep := time.Duration(2*maxRetryCount) * time.Second
		sleeper := &SpySleeper{}
		clock := &FixedClock{now: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)}

		config := responseConfig{
			statusCode:        429,
			responseBody:      "",
			retryAfter:        "2",
			includeRetryAfter: true,
		}
		serverState := &respSequenceServer{statuses: []responseConfig{config, config, config, config}}
		server := makeConfigurableServer(t, serverState)
		defer server.Close()

		want := ""
		got, err := getWeather(server.URL, sleeper, clock)
		if err == nil {
			t.Fatal("expected an error, got nil")
		}

		apiErr := requireAPIError(t, err)
		assertStatusCode(t, apiErr.StatusCode, http.StatusTooManyRequests)
		assertTimeSlept(t, sleeper.totalTimeSlept, totalRequiredSleep)
		assertWeatherString(t, got, want)
		assertNumRequests(t, serverState.requestCount, 4)
	})
}

func makeConfigurableServer(t testing.TB, s *respSequenceServer) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.requestCount >= len(s.statuses) {
			t.Fatalf("unexpected extra request")
		}

		response := s.statuses[s.requestCount]
		s.requestCount++

		if response.includeRetryAfter {
			w.Header().Set("Retry-After", response.retryAfter)
		}
		w.WriteHeader(response.statusCode)
		w.Write([]byte(response.responseBody))
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
