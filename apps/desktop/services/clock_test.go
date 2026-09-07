package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/raphaeldiscky/wails-template/apps/desktop/services"
)

// The whole point of ClockService is that its goroutine is owned and exits, so
// that is what these assert. A nil *application.App is safe here because both
// tests finish far inside the one-second tick interval, so Emit is never
// reached — if that ever changes, these will panic loudly rather than silently
// pass, which is the failure mode we want.
const shutdownBudget = 2 * time.Second

func TestClockServiceShutdownStopsTheGoroutine(t *testing.T) {
	t.Parallel()

	svc := services.NewClockService(nil)
	if err := svc.ServiceStartup(context.Background(), application.ServiceOptions{}); err != nil {
		t.Fatalf("ServiceStartup() error: %v", err)
	}

	// ServiceShutdown waits for the goroutine, so a hang here IS the leak.
	done := make(chan error, 1)
	go func() { done <- svc.ServiceShutdown() }()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("ServiceShutdown() error: %v", err)
		}
	case <-time.After(shutdownBudget):
		t.Fatal("ServiceShutdown() did not return: the ticker goroutine leaked")
	}
}

// Canceling the app context must also end the goroutine — otherwise a service
// whose shutdown hook never runs would keep ticking.
func TestClockServiceStopsOnContextCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	svc := services.NewClockService(nil)
	if err := svc.ServiceStartup(ctx, application.ServiceOptions{}); err != nil {
		t.Fatalf("ServiceStartup() error: %v", err)
	}

	cancel()

	// The goroutine should already be gone, so shutdown returns at once.
	done := make(chan error, 1)
	go func() { done <- svc.ServiceShutdown() }()

	select {
	case <-done:
	case <-time.After(shutdownBudget):
		t.Fatal("goroutine still running after the context was canceled")
	}
}
