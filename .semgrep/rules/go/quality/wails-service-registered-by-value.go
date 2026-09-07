package fixtures

import "github.com/wailsapp/wails/v3/pkg/application"

type DemoService struct{ calls int }

func byValue() {
	// ruleid: go-wails-service-registered-by-value
	_ = application.NewService(DemoService{})
}

func byValueWithOptions() {
	// ruleid: go-wails-service-registered-by-value
	_ = application.NewServiceWithOptions(DemoService{}, application.ServiceOptions{})
}

func byPointer() {
	// ok: go-wails-service-registered-by-value
	_ = application.NewService(&DemoService{})
}

func byConstructor() {
	// ok: go-wails-service-registered-by-value
	_ = application.NewService(newDemoService())
}

func newDemoService() *DemoService { return &DemoService{} }
