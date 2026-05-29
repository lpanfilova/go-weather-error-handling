package main

import (
	"io"
	"net/http"
)

func getWeather(clientURL string) (string, error) {
	resp, err := http.Get(clientURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	bodyString := string(bodyBytes)

	return bodyString, err
}
