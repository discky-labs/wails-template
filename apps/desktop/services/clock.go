package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// EventTime is the name of the tick event. Shared with main.go, which registers
// its payload type, so the name is written once.
const EventTime = "time"

// tickInterval is how often the clock emits.
const tickInterval = time.Second

// ClockService emits EventTime once a second, demonstrating Go -> JS events.
//
// It is also the worked example of goroutine ownership. The naive version of
// this — a bare `for { Emit; Sleep }` in main() — is the shape the Wails
// performance guide names as a memory leak: nothing stops it, so it outlives
// whatever it was serving. Here the goroutine has an owner (this service) and
// three ways out: the app context, an explicit stop, and shutdown waits for it.
type ClockService struct {
	app *application.App

	// stop is closed by ServiceShutdown; done is closed by the goroutine so
	// shutdown can wait for it rather than racing the process exit.
	stop chan struct{}
	done chan struct{}
}

// NewClockService wires the service to the app it emits through.
//
// A service that needs *application.App cannot be listed in
// application.Options (the app does not exist yet), so main.go registers it
// after construction with app.RegisterService — the pattern the Wails bindings
// best-practices guide documents for exactly this case.
func NewClockService(app *application.App) *ClockService {
	return &ClockService{
		app:  app,
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
}

// ServiceStartup starts the ticker goroutine.
func (s *ClockService) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	go s.tick(ctx)

	return nil
}

// ServiceShutdown stops the ticker and waits for the goroutine to exit.
func (s *ClockService) ServiceShutdown() error {
	close(s.stop)
	// Wait, so shutdown is deterministic: without this the goroutine could
	// still be mid-Emit while the runtime tears the event system down.
	<-s.done

	return nil
}

// tick emits until the context is canceled or ServiceShutdown is called.
func (s *ClockService) tick(ctx context.Context) {
	defer close(s.done)

	// Wails recovers panics in BOUND methods and in its own runtime — it does
	// NOT cover a background goroutine like this one. Without this recover a
	// single bad tick would take the whole process down.
	defer func() {
		if r := recover(); r != nil {
			slog.Error("clock goroutine panicked; ticks have stopped", "panic", r)
		}
	}()

	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.app.Event.Emit(EventTime, time.Now().Format(time.RFC1123))
		case <-ctx.Done():
			return
		case <-s.stop:
			return
		}
	}
}
