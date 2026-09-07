package fixtures

import "github.com/wailsapp/wails/v3/pkg/application"

func flatWindow(app *application.App) {
	// ruleid: go-wails-flat-manager-api
	app.NewWebviewWindowWithOptions(application.WebviewWindowOptions{})
}

func flatWindowBare(app *application.App) {
	// ruleid: go-wails-flat-manager-api
	app.NewWebviewWindow()
}

func flatEvent(app *application.App) {
	// ruleid: go-wails-flat-manager-api
	app.OnEvent("name", func(*application.CustomEvent) {})
}

func managerWindow(app *application.App) {
	// ok: go-wails-flat-manager-api
	app.Window.NewWithOptions(application.WebviewWindowOptions{})
}

func managerEvent(app *application.App) {
	// ok: go-wails-flat-manager-api
	app.Event.On("name", func(*application.CustomEvent) {})
}
