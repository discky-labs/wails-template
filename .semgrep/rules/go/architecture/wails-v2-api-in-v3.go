package fixtures

import (
	"context"

	// ruleid: go-wails-v2-api-in-v3
	v2runtime "github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type svc struct{ ctx context.Context }

func v2EventEmit(s *svc) {
	// ruleid: go-wails-v2-api-in-v3
	v2runtime.EventsEmit(s.ctx, "name", "data")
}

func v2BindOption() {
	// ruleid: go-wails-v2-api-in-v3
	_ = application.Options{
		Bind: []any{&svc{}},
	}
}

// ok: go-wails-v2-api-in-v3
func v3EventEmit(app *application.App) {
	app.Event.Emit("name", "data")
}

// ok: go-wails-v2-api-in-v3
func v3Services() {
	_ = application.Options{
		Services: []application.Service{
			application.NewService(&svc{}),
		},
	}
}
