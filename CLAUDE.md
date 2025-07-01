# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Grapeddit is a Go project that appears to be for Reddit API interaction and analysis. The project contains detailed documentation about Reddit's OAuth authentication flow and API methods used by clients like Redlib.

## Structure, Testing, and Mocking
Grapeddit uses dependency injection to feed in dependent data sources when instantiating structs. Always create
a separation between structs and interfaces so that mock structs can be created for testing purposes. Use `gomock`
to generate mocks when needed.

## Development Commands

### Go Commands
- `go run main.go` - Run the main application
- `go build` - Build the application
- `go mod tidy` - Clean up module dependencies
- `go test ./...` - Run all tests
- `go fmt ./...` - Format Go code

## Project Structure

- `main.go` - Entry point (currently minimal)
- `go.mod` - Go module definition
- `contrib/` - Documentation and analysis files
  - `analysis/CLIENT-ANALYSIS.md` - Detailed analysis of Reddit OAuth authentication flow and client behavior
  - `analysis/API-METHODS.md` - Documentation of Reddit API endpoints and methods
  - `analysis/FETCHING-POST-COMMENTS.md` - Documentation of Reddit API endpoints and methods for fetching post comments

## Key Technical Context

The project focuses on Reddit API interaction with emphasis on:
- OAuth authentication spoofing Reddit Android client
- Rate limiting and token management
- Header manipulation and anti-fingerprinting
- Content restriction handling (gated, quarantined, private)
- URL sanitization and canonicalization

The documentation in `contrib/` provides critical context for understanding Reddit's API patterns and client behavior that would be essential for implementing the actual Go code.
