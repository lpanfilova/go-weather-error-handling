package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetWeather(t *testing.T) {
	t.Run("returns weather when server responds 200 OK", func(t *testing.T) {
		server := makeServer("sunny", http.StatusOK)

		defer server.Close()

		want := "sunny"
		got, err := getWeather(server.URL)
		if err != nil {
			t.Fatalf("failed to call server: %v", err)
		}

		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	})

	t.Run("returns error when server responds 500", func(t *testing.T) {
		server := makeServer("sunny", http.StatusInternalServerError)
		defer server.Close()

		want := ""
		got, err := getWeather(server.URL)

		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}

		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func makeServer(body string, status int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
}
