// Package services holds the Wails-bound adapters.
//
// Each exported method becomes a JS/TS function via `wails3 generate bindings`.
// Keep these types thin: they validate input, call into Wails-free packages
// (libraries/core), and map results to values the frontend can consume. Domain
// logic does not belong here, so it stays testable without a webview.
package services

import (
	"errors"
	"fmt"
	"strings"

	"github.com/raphaeldiscky/wails-template/libraries/core"
)

// ErrEmptyName is returned when no name is supplied to Greet.
var ErrEmptyName = errors.New("name must not be empty")

// maxNameLen bounds the greeting so a hostile or accidental megabyte of input
// can't be echoed back into the UI.
const maxNameLen = 64

// DemoService demonstrates the two shapes a bound method can take: one that
// always succeeds, and one that can fail.
//
// Services are singletons shared across every window, so any mutable state
// added here must be guarded by a mutex.
type DemoService struct{}

// Greet returns a greeting for name.
//
// The frontend is untrusted input — it is validated here exactly as an HTTP
// handler would validate a request body.
func (s *DemoService) Greet(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", ErrEmptyName
	}

	if len(trimmed) > maxNameLen {
		return "", fmt.Errorf("name must be at most %d characters", maxNameLen)
	}

	return "Hello " + trimmed + "!", nil
}

// Divide exposes core.Divide so the frontend has a bound method that genuinely
// fails: a returned error arrives in JS as a rejected promise.
func (s *DemoService) Divide(a, b int) (int, error) {
	result, err := core.Divide(a, b)
	if err != nil {
		return 0, fmt.Errorf("divide %d by %d: %w", a, b, err)
	}

	return result, nil
}
