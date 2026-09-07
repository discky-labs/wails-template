package config_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/raphaeldiscky/wails-template/libraries/config"
)

func storeInTemp(t *testing.T) *config.Store {
	t.Helper()

	return config.NewStoreAt(filepath.Join(t.TempDir(), "prefs", "preferences.json"))
}

// A first run has no file. That must read as defaults, not as an error, or the
// app would refuse to start until something wrote one.
func TestLoadMissingFileYieldsDefaults(t *testing.T) {
	t.Parallel()

	got, err := storeInTemp(t).Load()
	if err != nil {
		t.Fatalf("Load() on a missing file returned error: %v", err)
	}

	if got != config.Default() {
		t.Errorf("Load() = %+v, want defaults %+v", got, config.Default())
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	t.Parallel()

	store := storeInTemp(t)
	want := config.Preferences{Theme: config.ThemeDark, WindowWidth: 1280, WindowHeight: 800}

	if err := store.Save(want); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if got != want {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}

// Save creates the directory tree; nothing else in the app does it, so a first
// run on a clean machine depends on this.
func TestSaveCreatesDirectory(t *testing.T) {
	t.Parallel()

	store := storeInTemp(t)
	if err := store.Save(config.Default()); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	if _, err := os.Stat(store.Path()); err != nil {
		t.Errorf("preferences file not written: %v", err)
	}
}

// The file may hold identifying settings, so it must not be group/world
// readable. Skipped on Windows, where Unix mode bits are not meaningful.
func TestSaveUsesOwnerOnlyPermissions(t *testing.T) {
	t.Parallel()

	if os.PathSeparator == '\\' {
		t.Skip("POSIX permission bits are not meaningful on Windows")
	}

	store := storeInTemp(t)
	if err := store.Save(config.Default()); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	info, err := os.Stat(store.Path())
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}

	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file mode = %#o, want 0600", perm)
	}
}

// A corrupt file must not brick the app: report the problem AND hand back
// something usable.
func TestLoadCorruptFileReturnsDefaultsAndError(t *testing.T) {
	t.Parallel()

	store := storeInTemp(t)
	if err := os.MkdirAll(filepath.Dir(store.Path()), 0o700); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	if err := os.WriteFile(store.Path(), []byte("{not json"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	got, err := store.Load()
	if err == nil {
		t.Error("Load() on a corrupt file returned nil error, want one")
	}

	if got != config.Default() {
		t.Errorf("Load() = %+v, want defaults %+v", got, config.Default())
	}
}

// One bad field should not cost the user every other setting.
func TestLoadRepairsOutOfRangeValues(t *testing.T) {
	t.Parallel()

	store := storeInTemp(t)
	if err := os.MkdirAll(filepath.Dir(store.Path()), 0o700); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	raw, err := json.Marshal(map[string]any{
		"theme":        "chartreuse",
		"windowWidth":  1,
		"windowHeight": 999999,
	})
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}

	if err := os.WriteFile(store.Path(), raw, 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if got.Theme != config.Default().Theme {
		t.Errorf("Theme = %q, want the default %q", got.Theme, config.Default().Theme)
	}

	if got.WindowWidth != 480 {
		t.Errorf("WindowWidth = %d, want it clamped to 480", got.WindowWidth)
	}

	if got.WindowHeight != 10000 {
		t.Errorf("WindowHeight = %d, want it clamped to 10000", got.WindowHeight)
	}
}

func TestNormaliseReportsWhetherItChangedAnything(t *testing.T) {
	t.Parallel()

	unchanged := config.Default()
	if unchanged.Normalise() {
		t.Error("Normalise() on defaults reported a change, want none")
	}

	broken := config.Preferences{Theme: "nope", WindowWidth: 0, WindowHeight: 0}
	if !broken.Normalise() {
		t.Error("Normalise() on a broken value reported no change, want one")
	}
}

func TestSetTheme(t *testing.T) {
	t.Parallel()

	var prefs config.Preferences
	if err := prefs.SetTheme(config.ThemeDark); err != nil {
		t.Fatalf("SetTheme(ThemeDark) error: %v", err)
	}

	if prefs.Theme != config.ThemeDark {
		t.Errorf("Theme = %q, want ThemeDark", prefs.Theme)
	}

	err := prefs.SetTheme("chartreuse")
	if !errors.Is(err, config.ErrUnknownTheme) {
		t.Errorf("SetTheme(chartreuse) error = %v, want ErrUnknownTheme", err)
	}
}

func TestNewStoreRejectsEmptyAppName(t *testing.T) {
	t.Parallel()

	if _, err := config.NewStore(""); err == nil {
		t.Error("NewStore(\"\") returned nil error, want one")
	}
}
