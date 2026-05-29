package main

import (
	"errors"
	"io"
	"net/http"
)

func getWeather(clientURL string) (string, error) {
	resp, err := http.Get(clientURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 500 {
		return "", errors.New("Server is unavailable: status code 500")
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	bodyString := string(bodyBytes)

	return bodyString, err
}
