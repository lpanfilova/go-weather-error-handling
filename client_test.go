package main

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

type SpyWaiter struct {
	durationWaited time.Duration
}

func (s *SpyWaiter) Wait(duration time.Duration) {
	s.durationWaited = duration
}

func TestGetWeather(t *testing.T) {
	t.Run("returns weather when server responds 200 OK", func(t *testing.T) {
		waiter := &SpyWaiter{}
		server := makeServer("sunny", http.StatusOK)

		defer server.Close()

		want := "sunny"
		got, err := getWeather(server.URL, waiter)
		if err != nil {
			t.Fatalf("failed to call server: %v", err)
		}

		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	})

	t.Run("returns error when server responds 500", func(t *testing.T) {
		waiter := &SpyWaiter{}
		server := makeServer("sunny", http.StatusInternalServerError)
		defer server.Close()

		want := ""
		got, err := getWeather(server.URL, waiter)

		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}

		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("retries after 'Retry-After' seconds if a weather server returns 429", func(t *testing.T) {
		timeToWait := 2
		server := makeRetryServer("sunny", timeToWait)
		defer server.Close()

		waiter := &SpyWaiter{}

		want := "sunny"
		got, err := getWeather(server.URL, waiter)
		if err != nil {
			t.Fatalf("server failed to retry: %v", err)
		}

		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}

		if waiter.durationWaited != time.Duration(timeToWait)*time.Second {
			t.Errorf("waited %v seconds, want to wait %v seconds", waiter.durationWaited, 1)
		}
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
