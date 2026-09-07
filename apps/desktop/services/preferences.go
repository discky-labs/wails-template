package services

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/raphaeldiscky/wails-template/libraries/config"
)

// PreferencesService exposes the persisted user preferences to the frontend.
//
// It is the worked example of the two service rules that are easy to get wrong:
//
//   - **Lifecycle.** ServiceStartup loads the file once, ServiceShutdown writes
//     it back. Doing the read in a bound method instead would hit the disk on
//     every call from the UI.
//   - **Shared state.** A service is a singleton shared across every window, so
//     several windows can call Get and SetTheme concurrently. Every access to
//     `prefs` goes through the mutex.
type PreferencesService struct {
	store *config.Store

	// mu guards prefs and is NEVER held across disk I/O: a bound method is
	// reachable from every window, so a reader must not queue behind a file
	// write. writeMu serializes the read-modify-write instead, which keeps two
	// concurrent writers from interleaving on the file without ever making
	// Get() wait on a syscall.
	mu      sync.RWMutex
	prefs   config.Preferences
	writeMu sync.Mutex
}

// NewPreferencesService wires the service to a store. Dependencies arrive
// through a plain constructor — there is no DI framework.
func NewPreferencesService(store *config.Store) *PreferencesService {
	return &PreferencesService{store: store, prefs: config.Default()}
}

// ServiceStartup loads the preferences file.
//
// Returning an error here would abort application startup, which is too harsh
// for a preferences file: a missing one is a first run, and a corrupt one is
// recoverable because Load hands back usable defaults. So the problem is logged
// and the app starts.
func (s *PreferencesService) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	prefs, err := s.store.Load()
	if err != nil {
		slog.Warn("could not read preferences; continuing with defaults",
			"path", s.store.Path(), "error", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.prefs = prefs

	return nil
}

// ServiceShutdown persists the preferences.
//
// Services shut down in reverse registration order and the app context is
// already canceled, so this must not depend on anything else still running.
// Writing a file does not.
func (s *PreferencesService) ServiceShutdown() error {
	// Take writeMu so this cannot interleave with an in-flight SetTheme.
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	s.mu.RLock()
	prefs := s.prefs
	s.mu.RUnlock()

	if err := s.store.Save(prefs); err != nil {
		return fmt.Errorf("persist preferences: %w", err)
	}

	return nil
}

// Get returns the current preferences.
func (s *PreferencesService) Get() config.Preferences {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.prefs
}

// SetTheme validates and stores the theme, then writes it through immediately.
//
// Write-through rather than waiting for shutdown: a desktop app can be killed
// or crash, and silently losing a setting the user just changed is worse than
// the cost of one small file write.
func (s *PreferencesService) SetTheme(theme string) (config.Preferences, error) {
	// writeMu spans the whole read-modify-write, so two callers cannot each
	// read the old value and race to persist their own version.
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	s.mu.RLock()
	updated := s.prefs
	s.mu.RUnlock()

	if err := updated.SetTheme(theme); err != nil {
		return s.Get(), fmt.Errorf("set theme: %w", err)
	}

	// The write happens with mu released — readers keep running.
	if err := s.store.Save(updated); err != nil {
		return s.Get(), fmt.Errorf("persist preferences: %w", err)
	}

	s.mu.Lock()
	s.prefs = updated
	s.mu.Unlock()

	return updated, nil
}

// Themes lists the accepted theme names, so the frontend renders the options
// from one source rather than hardcoding a list that can drift.
func (s *PreferencesService) Themes() []string {
	return config.Themes()
}
