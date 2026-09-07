package services_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/raphaeldiscky/wails-template/libraries/core"

	"github.com/raphaeldiscky/wails-template/apps/desktop/services"
)

// The frontend is untrusted input, so the validation in Greet is the boundary —
// these are the cases a UI can actually send.
const helloAda = "Hello Ada!"

func TestGreet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{name: "plain name", input: "Ada", want: helloAda},
		{name: "trims surrounding space", input: "  Ada  ", want: helloAda},
		{name: "empty is rejected", input: "", wantErr: services.ErrEmptyName},
		{name: "whitespace only is rejected", input: "   ", wantErr: services.ErrEmptyName},
	}

	var svc services.DemoService

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := svc.Greet(tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Greet(%q) error = %v, want %v", tt.input, err, tt.wantErr)
			}

			if got != tt.want {
				t.Errorf("Greet(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// An over-long name is rejected rather than echoed back into the UI.
func TestGreetRejectsOverlongName(t *testing.T) {
	t.Parallel()

	var svc services.DemoService

	if _, err := svc.Greet(strings.Repeat("a", 65)); err == nil {
		t.Error("Greet() with a 65-character name returned nil error, want one")
	}

	if _, err := svc.Greet(strings.Repeat("a", 64)); err != nil {
		t.Errorf("Greet() with a 64-character name errored: %v", err)
	}
}

// Divide must surface the domain error, wrapped so its origin is identifiable
// but still matchable with errors.Is by the caller.
func TestDivide(t *testing.T) {
	t.Parallel()

	var svc services.DemoService

	got, err := svc.Divide(10, 2)
	if err != nil {
		t.Fatalf("Divide(10, 2) error: %v", err)
	}

	if got != 5 {
		t.Errorf("Divide(10, 2) = %d, want 5", got)
	}

	if _, err := svc.Divide(1, 0); !errors.Is(err, core.ErrDivisionByZero) {
		t.Errorf("Divide(1, 0) error = %v, want core.ErrDivisionByZero", err)
	}
}
