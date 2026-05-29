# Weather Client in Go

A Go command-line weather client built from scratch to practice practical error handling and test-driven development.

The client calls an unreliable weather server and handles:

- successful weather responses
- temporary rate limits
- retry-after delays
- invalid responses
- server/network failures
- stdout vs stderr
- exit codes

## Project goals

This project focuses on:

- writing a small CLI application in Go
- practicing TDD outside of a course repository
- handling HTTP errors clearly

## Attribution

The project brief and external test server come from CodeYourFuture's `immersive-go-course`.

This repository contains my own implementation of the client. The external server is used only as a local test target.

## Running locally

1. Start the CYF test server:

   ```bash
   git clone https://github.com/CodeYourFuture/immersive-go-course.git
   cd immersive-go-course/projects/output-and-error-handling/server
   go run .