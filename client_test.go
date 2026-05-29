package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetWeather(t *testing.T) {
	t.Run("returns weather when server responds 200 OK", func(t *testing.T) {
		server := makeServer("sunny")

		defer server.Close()

		want := "sunny"
		got, err := getWeather(server.URL)
		if err != nil {
			t.Fatalf("Failed to call server: %v", err)
		}

		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	})
}

func makeServer(weather string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(weather))
	}))
}
