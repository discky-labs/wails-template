package main

import (
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime/trace"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/raphaeldiscky/wails-template/libraries/config"

	"github.com/raphaeldiscky/wails-template/apps/desktop/services"
)

// appName is used for the application metadata and the window title.
const appName = "desktop"

// traceFile is where an opt-in execution trace is written.
const traceFile = "trace.out"

// The production bundle is embedded, so the binary ships without loose assets.
// The `all:` prefix matters — without it, Go silently skips files Vite emits
// with a leading underscore or dot.
//
// In dev mode the asset server ignores this and proxies to the Vite server
// named by FRONTEND_DEVSERVER_URL (see scripts/wails-dev.sh); the directory
// still has to exist for this directive to compile, which is why
// frontend/dist/.gitkeep is tracked.
//
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Opt-in execution trace. Off by default so it costs nothing in normal
	// runs; see .claude/skills/wails-best-practices for how to read it.
	stopTrace := func() {}

	if os.Getenv("WAILS_TRACE") == "1" {
		stop, err := startTrace()
		if err != nil {
			slog.Error("could not start trace; continuing untraced", "error", err)
		} else {
			stopTrace = stop

			slog.Info("execution trace started", "file", traceFile)
		}
	}

	// Registering an event's payload type makes the binding generator emit a
	// typed listener for it on the frontend.
	application.RegisterEvent[string](services.EventTime)

	// Preferences live in the OS config dir. A failure here is fatal: it means
	// the platform has no config directory, so there is nowhere to persist to.
	prefsStore, err := config.NewStore(appName)
	if err != nil {
		slog.Error("cannot locate the user config directory", "error", err)
		stopTrace()
		os.Exit(1)
	}

	app := application.New(application.Options{
		Name:        appName,
		Description: "Wails v3 + Go + React desktop template",
		Services: []application.Service{
			application.NewService(&services.DemoService{}),
			// Registration order is startup order, and shutdown runs in
			// reverse — so this service loads last and saves first.
			application.NewService(services.NewPreferencesService(prefsStore)),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		// Wails recovers panics in bound methods and in its own runtime, then
		// hands them here. The DEFAULT handler prints a raw banner and quits;
		// this one logs structured detail and lets the app continue, because
		// the panic has already been contained and killing the window would
		// lose the user's unsaved state. Call app.Quit() here instead if you
		// would rather fail hard.
		PanicHandler: func(details *application.PanicDetails) {
			slog.Error("recovered panic",
				"error", details.Error,
				"at", details.Time,
				"stack", details.StackTrace)
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	// Needs the app itself, which cannot be referenced inside the Options
	// above, so it is registered after construction.
	app.RegisterService(application.NewService(services.NewClockService(app)))

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  appName,
		Width:  1000,
		Height: 618,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		// MUST equal --background in frontend/src/styles/tokens.css, or the
		// native window flashes the wrong color before the webview paints.
		// Enforced by scripts/check-design-tokens.mjs.
		BackgroundColour: application.NewRGB(10, 10, 10),
		URL:              "/",
	})

	// Run blocks on the main goroutine until the application exits.
	runErr := app.Run()

	// Explicit, not deferred: os.Exit below would skip a defer, and a truncated
	// trace file is worse than none.
	stopTrace()

	if runErr != nil {
		slog.Error("application exited with an error", "error", runErr)
		os.Exit(1)
	}
}

// startTrace begins an execution trace, returning the function that ends it.
func startTrace() (func(), error) {
	file, err := os.Create(traceFile)
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", traceFile, err)
	}

	if err := trace.Start(file); err != nil {
		// Join rather than drop: a close failure here can point at the real
		// cause (a full disk). errors.Join ignores a nil.
		return nil, errors.Join(
			fmt.Errorf("start trace: %w", err),
			file.Close(),
		)
	}

	return func() {
		trace.Stop()

		if closeErr := file.Close(); closeErr != nil {
			slog.Error("could not close the trace file", "file", traceFile, "error", closeErr)
		}
	}, nil
}
