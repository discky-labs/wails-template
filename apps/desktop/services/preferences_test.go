package services_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/raphaeldiscky/wails-template/libraries/config"

	"github.com/raphaeldiscky/wails-template/apps/desktop/services"
)

func newService(t *testing.T) *services.PreferencesService {
	t.Helper()

	store := config.NewStoreAt(filepath.Join(t.TempDir(), "preferences.json"))

	return services.NewPreferencesService(store)
}

// A missing file is a first run, so startup must succeed and expose defaults.
func TestServiceStartupOnFirstRun(t *testing.T) {
	t.Parallel()

	svc := newService(t)
	if err := svc.ServiceStartup(context.Background(), application.ServiceOptions{}); err != nil {
		t.Fatalf("ServiceStartup() error: %v", err)
	}

	if got := svc.Get(); got != config.Default() {
		t.Errorf("Get() = %+v, want defaults %+v", got, config.Default())
	}
}

func TestSetThemePersistsAcrossRestart(t *testing.T) {
	t.Parallel()

	store := config.NewStoreAt(filepath.Join(t.TempDir(), "preferences.json"))
	svc := services.NewPreferencesService(store)

	if _, err := svc.SetTheme(config.ThemeDark); err != nil {
		t.Fatalf("SetTheme() error: %v", err)
	}

	// A second service over the same file stands in for the next app launch —
	// SetTheme writes through, so this must see it without a shutdown.
	restarted := services.NewPreferencesService(store)
	if err := restarted.ServiceStartup(
		context.Background(),
		application.ServiceOptions{},
	); err != nil {
		t.Fatalf("ServiceStartup() error: %v", err)
	}

	if got := restarted.Get().Theme; got != config.ThemeDark {
		t.Errorf("Theme after restart = %q, want ThemeDark", got)
	}
}

// A rejected theme must leave the stored value untouched, and the returned
// preferences must still be the real ones so the UI can resync from them.
func TestSetThemeRejectsUnknownAndKeepsCurrent(t *testing.T) {
	t.Parallel()

	svc := newService(t)
	if _, err := svc.SetTheme(config.ThemeDark); err != nil {
		t.Fatalf("SetTheme(dark) error: %v", err)
	}

	got, err := svc.SetTheme("chartreuse")
	if !errors.Is(err, config.ErrUnknownTheme) {
		t.Errorf("SetTheme(chartreuse) error = %v, want ErrUnknownTheme", err)
	}

	if got.Theme != config.ThemeDark {
		t.Errorf("returned Theme = %q, want the unchanged dark", got.Theme)
	}

	if svc.Get().Theme != config.ThemeDark {
		t.Errorf("stored Theme = %q, want the unchanged dark", svc.Get().Theme)
	}
}

func TestServiceShutdownWritesTheFile(t *testing.T) {
	t.Parallel()

	store := config.NewStoreAt(filepath.Join(t.TempDir(), "preferences.json"))
	svc := services.NewPreferencesService(store)

	if err := svc.ServiceShutdown(); err != nil {
		t.Fatalf("ServiceShutdown() error: %v", err)
	}

	if _, err := store.Load(); err != nil {
		t.Errorf("preferences not readable after shutdown: %v", err)
	}
}

// Get and SetTheme are reachable from every window at once; -race turns a
// missing mutex here into a failure rather than a rare production glitch.
func TestConcurrentAccessIsRaceFree(t *testing.T) {
	t.Parallel()

	svc := newService(t)
	errs := make(chan error, 50)

	go func() {
		defer close(errs)

		for range 50 {
			if _, err := svc.SetTheme(config.ThemeLight); err != nil {
				errs <- err
			}
		}
	}()

	for range 50 {
		_ = svc.Get()
	}

	for err := range errs {
		t.Errorf("concurrent SetTheme: %v", err)
	}
}

func TestThemes(t *testing.T) {
	t.Parallel()

	if got := newService(t).Themes(); len(got) != len(config.Themes()) {
		t.Errorf("Themes() = %v, want %v", got, config.Themes())
	}
}
