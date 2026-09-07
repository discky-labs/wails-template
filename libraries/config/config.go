// Package config persists user preferences to the OS config directory.
//
// It imports no Wails package: the store is plain Go, so it is testable against
// a temp directory and the Wails service in apps/desktop/services is a thin
// adapter over it.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
)

// File permissions: preferences can carry user-identifying settings, so the
// file is owner-only and the directory is not world-listable.
const (
	filePerm = 0o600
	dirPerm  = 0o700

	fileName = "preferences.json"
)

// Window bounds are clamped so a preferences file written by an older version —
// or hand-edited to something absurd — cannot produce an unusable window.
const (
	minWindowSide = 480
	maxWindowSide = 10000
)

// The accepted theme names. Exported so the frontend receives them through the
// generated bindings instead of hardcoding a list that can drift.
const (
	ThemeSystem = "system"
	ThemeLight  = "light"
	ThemeDark   = "dark"
)

// Themes returns the accepted theme names.
//
// A function rather than a package-level slice: an exported slice is mutable by
// any caller, so one append elsewhere would silently change what this package
// accepts.
func Themes() []string {
	return []string{ThemeSystem, ThemeLight, ThemeDark}
}

// ErrUnknownTheme is returned when a theme is not one of [Themes].
var ErrUnknownTheme = errors.New("unknown theme")

// Preferences is the user-controlled application state that outlives a run.
type Preferences struct {
	Theme        string `json:"theme"`
	WindowWidth  int    `json:"windowWidth"`
	WindowHeight int    `json:"windowHeight"`
}

// Default returns the preferences a first run starts from. The window size
// matches the one main.go opens with.
func Default() Preferences {
	return Preferences{
		Theme:        ThemeSystem,
		WindowWidth:  1000,
		WindowHeight: 618,
	}
}

// Normalise repairs values that are out of range rather than rejecting the
// whole file: a single bad field should not cost the user every other setting.
// It reports whether anything changed, so a caller can decide to rewrite.
func (p *Preferences) Normalise() bool {
	original := *p

	if !slices.Contains(Themes(), p.Theme) {
		p.Theme = Default().Theme
	}

	p.WindowWidth = clamp(p.WindowWidth, minWindowSide, maxWindowSide, Default().WindowWidth)
	p.WindowHeight = clamp(p.WindowHeight, minWindowSide, maxWindowSide, Default().WindowHeight)

	return *p != original
}

// clamp keeps v within [lo, hi], falling back to fallback when v is unset.
func clamp(v, lo, hi, fallback int) int {
	if v == 0 {
		return fallback
	}

	if v < lo {
		return lo
	}

	if v > hi {
		return hi
	}

	return v
}

// Store reads and writes a preferences file.
type Store struct {
	path string
}

// NewStore returns a Store under the user's OS config directory —
// ~/.config/<appName> on Linux, ~/Library/Application Support/<appName> on
// macOS, %AppData%\<appName> on Windows.
func NewStore(appName string) (*Store, error) {
	if appName == "" {
		return nil, errors.New("config: appName must not be empty")
	}

	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("config: locate user config dir: %w", err)
	}

	return NewStoreAt(filepath.Join(dir, appName, fileName)), nil
}

// NewStoreAt returns a Store for an explicit path. Tests use it with t.TempDir.
func NewStoreAt(path string) *Store {
	return &Store{path: path}
}

// Path reports the file the Store reads and writes.
func (s *Store) Path() string {
	return s.path
}

// Load reads the preferences file.
//
// A missing file is not an error — it is a first run, and yields [Default].
// A corrupt file is also not fatal: it is reported alongside the defaults so
// the caller can carry on with a usable app and log the problem.
func (s *Store) Load() (Preferences, error) {
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}

	if err != nil {
		return Default(), fmt.Errorf("config: read %s: %w", s.path, err)
	}

	prefs := Default()
	if err := json.Unmarshal(raw, &prefs); err != nil {
		return Default(), fmt.Errorf("config: parse %s: %w", s.path, err)
	}

	prefs.Normalise()

	return prefs, nil
}

// Save writes the preferences file atomically.
//
// The write goes to a temp file in the same directory and is then renamed:
// rename within one filesystem is atomic, so a crash or a full disk mid-write
// leaves the previous file intact rather than a truncated one.
func (s *Store) Save(prefs Preferences) error {
	prefs.Normalise()

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return fmt.Errorf("config: create %s: %w", dir, err)
	}

	encoded, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return fmt.Errorf("config: encode preferences: %w", err)
	}

	temp, err := os.CreateTemp(dir, fileName+".*")
	if err != nil {
		return fmt.Errorf("config: create temp file in %s: %w", dir, err)
	}

	tempName := temp.Name()
	// Cleanup for the paths that return early. After a successful rename the
	// temp path is gone, so ErrNotExist is the expected outcome and is not
	// worth reporting; anything else means a leftover file, which is.
	defer func() {
		if err := os.Remove(tempName); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Debug("could not remove temporary preferences file",
				"path", tempName, "error", err)
		}
	}()

	if err := writeAndClose(temp, encoded); err != nil {
		return err
	}

	if err := os.Chmod(tempName, filePerm); err != nil {
		return fmt.Errorf("config: chmod %s: %w", tempName, err)
	}

	if err := os.Rename(tempName, s.path); err != nil {
		return fmt.Errorf("config: replace %s: %w", s.path, err)
	}

	return nil
}

// writeAndClose writes data and closes f, reporting whichever step failed.
// Close is checked because a buffered write can only surface ENOSPC there.
func writeAndClose(file *os.File, data []byte) error {
	if _, err := file.Write(data); err != nil {
		// Join rather than drop: the write error is the cause, but a close
		// error alongside it can point at the real problem (a full disk).
		// errors.Join ignores a nil, so a clean close adds nothing.
		return errors.Join(
			fmt.Errorf("config: write %s: %w", file.Name(), err),
			file.Close(),
		)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("config: close %s: %w", file.Name(), err)
	}

	return nil
}

// SetTheme validates and sets the theme.
func (p *Preferences) SetTheme(theme string) error {
	if !slices.Contains(Themes(), theme) {
		return fmt.Errorf("%w: %q (want one of %v)", ErrUnknownTheme, theme, Themes())
	}

	p.Theme = theme

	return nil
}
