package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/trace"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/raphaeldiscky/wails-template/libraries/config"

	"github.com/raphaeldiscky/wails-template/apps/desktop/internal/repository"
	"github.com/raphaeldiscky/wails-template/apps/desktop/services"
)

// appName is used for the application metadata and the window title.
const appName = "desktop"

// traceFile is where an opt-in execution trace is written.
const traceFile = "trace.out"

// dbFileName sits beside preferences.json in the OS config directory.
const dbFileName = "desktop.db"

// Window geometry. Matches config.Default so a first run opens at the size the
// preferences file will report.
const (
	windowWidth  = 1000
	windowHeight = 618
)

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

// main does nothing but translate an error into an exit status.
//
// Everything else is in run, and that split is the point: os.Exit does not run
// deferred functions, so any cleanup written next to an os.Exit call is
// cleanup that silently never happens. Keeping the only os.Exit here means
// every defer in run is honored.
func main() {
	if err := run(); err != nil {
		slog.Error("desktop exited with an error", "error", err)
		os.Exit(1)
	}
}

// run is the composition root: the one place that constructs dependencies and
// the one place that releases them.
//
// The order below is fixed, and shutdown is its exact reverse because defers
// unwind LIFO:
//
//  1. Platform — the opt-in execution trace.
//  2. Paths — where this machine keeps our data.
//  3. Backing stores — the database and the preferences file.
//  4. Services — plain constructors over those stores.
//  5. Application and window.
//  6. Run, which blocks until the app exits.
//
// Anything that needs releasing is deferred the moment it is acquired, so a
// failure at step 4 still closes what step 3 opened.
func run() error {
	// Step 1. Off by default, so it costs nothing in a normal run; see
	// .claude/skills/wails-best-practices for how to read the output.
	if os.Getenv("WAILS_TRACE") == "1" {
		stop, err := startTrace()
		if err != nil {
			slog.Error("could not start trace; continuing untraced", "error", err)
		} else {
			defer stop()

			slog.Info("execution trace started", "file", traceFile)
		}
	}

	// Registering an event's payload type makes the binding generator emit a
	// typed listener for it on the frontend.
	application.RegisterEvent[string](services.EventTime)

	// Step 2. A failure here means the platform has no config directory, so
	// there is nowhere to persist to at all.
	prefsStore, err := config.NewStore(appName)
	if err != nil {
		return fmt.Errorf("locate the user config directory: %w", err)
	}

	// Derived from the preferences path rather than recomputed, so the two
	// files cannot drift into different directories.
	dbPath := filepath.Join(filepath.Dir(prefsStore.Path()), dbFileName)

	// Step 3. Opening the database also migrates it.
	ctx := context.Background()

	store, err := repository.Open(ctx, dbPath)
	if err != nil {
		return fmt.Errorf("open the notes database: %w", err)
	}

	// Deferred here, immediately after acquiring it: this unwinds AFTER
	// app.Run returns, which is after every ServiceShutdown has had its last
	// chance to write.
	defer func() {
		if closeErr := store.Close(); closeErr != nil {
			slog.Error("could not close the database", "path", dbPath, "error", closeErr)
		}
	}()

	// Steps 4 and 5.
	app := application.New(application.Options{
		Name:        appName,
		Description: "Wails v3 + Go + React desktop template",
		Services: []application.Service{
			application.NewService(&services.DemoService{}),
			application.NewService(services.NewNotesService(store)),
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
		Width:  windowWidth,
		Height: windowHeight,
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

	// Step 6. Blocks on the main goroutine until the application exits.
	if err := app.Run(); err != nil {
		return fmt.Errorf("run the application: %w", err)
	}

	return nil
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
