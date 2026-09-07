package main

import (
	"embed"
	"log/slog"
	"os"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/raphaeldiscky/wails-template/libraries/config"

	"github.com/raphaeldiscky/wails-template/apps/desktop/services"
)

// The production bundle is embedded, so the binary ships without loose assets.
// The `all:` prefix matters — without it, files Vite emits with a leading
// underscore or dot are silently left out.
//
// In dev mode the asset server ignores this and proxies to the Vite server
// named by FRONTEND_DEVSERVER_URL (see scripts/wails-dev.sh); the directory
// still has to exist for this directive to compile.
//
//go:embed all:frontend/dist
var assets embed.FS

// appName is used for the application metadata and the window title.
const appName = "desktop"

func main() {
	// Registering an event's payload type makes the binding generator emit a
	// typed listener for it on the frontend.
	application.RegisterEvent[string]("time")

	// Preferences live in the OS config dir. A failure here is fatal: it means
	// the platform has no config directory, so there is nowhere to persist to.
	prefsStore, err := config.NewStore(appName)
	if err != nil {
		slog.Error("cannot locate the user config directory", "error", err)
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
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

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

	// Emit a tick the frontend listens for, demonstrating Go -> JS events.
	go func() {
		for {
			app.Event.Emit("time", time.Now().Format(time.RFC1123))
			time.Sleep(time.Second)
		}
	}()

	// Run blocks on the main goroutine until the app exits.
	if err := app.Run(); err != nil {
		slog.Error("application exited with an error", "error", err)
		os.Exit(1)
	}
}
