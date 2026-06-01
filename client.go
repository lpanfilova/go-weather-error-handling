package main

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"
)

type Waiter interface {
	Wait(time.Duration)
}

type RealWaiter struct{}

func (s *RealWaiter) Wait(duration time.Duration) {
	time.Sleep(duration)
}

func getWeather(clientURL string, w Waiter) (string, error) {
	var weather string

	resp, err := http.Get(clientURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		weather = string(bodyBytes)

	case 500:
		return "", errors.New("Server is unavailable: status code 500")
	case 429:
		seconds, err := strconv.Atoi(resp.Header.Get("Retry-After"))
		if err != nil {
			return "", err
		}
		timeToWait := time.Duration(seconds) * time.Second
		w.Wait(timeToWait)
		weather, err = getWeather(clientURL, w)
	}

	return weather, err
}
