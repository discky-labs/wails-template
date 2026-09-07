package services

import (
	"context"
	"fmt"

	"github.com/raphaeldiscky/wails-template/apps/desktop/internal/entity"
	"github.com/raphaeldiscky/wails-template/apps/desktop/internal/repository"
)

// NotesService exposes the persisted notes to the frontend.
//
// It is the worked example of a service backed by a database, and shows the
// three things such a service does and nothing else:
//
//   - **Validate.** The frontend is untrusted, so a note is checked and a page
//     size clamped before either reaches the database.
//   - **Call the DataStore.** No SQL, no driver import, no transaction
//     plumbing — those live in internal/repository, which owns them.
//   - **Map errors.** A repository returns entity sentinels; this layer wraps
//     them for the UI and never leaks a driver error.
//
// Unlike PreferencesService there is no mutex here, and none is needed: the
// shared state is the database, and database/sql is safe for concurrent use.
// A service is a singleton across every window, so that distinction matters.
type NotesService struct {
	store repository.DataStore
}

// NewNotesService wires the service to a DataStore. A plain constructor, as
// everywhere else in this package — there is no DI framework.
func NewNotesService(store repository.DataStore) *NotesService {
	return &NotesService{store: store}
}

// Add validates and stores one note.
//
// The ctx is injected by Wails and does NOT appear in the generated TypeScript
// signature — the frontend calls Add(title, body). It carries the cancellation
// of the frontend's promise, so the query stops when the caller stops waiting.
func (s *NotesService) Add(ctx context.Context, title, body string) (entity.Note, error) {
	note := entity.Note{Title: title, Body: body}

	if err := note.Validate(); err != nil {
		return entity.Note{}, fmt.Errorf("add note: %w", err)
	}

	stored, err := s.store.Notes().Insert(ctx, note)
	if err != nil {
		return entity.Note{}, fmt.Errorf("add note: %w", err)
	}

	return stored, nil
}

// AddMany stores several notes in one transaction.
//
// This is the first rule of the bindings performance guide — batch, so one
// bridge crossing carries N notes instead of N crossings carrying one — and it
// is why Atomic exists: either the whole batch lands or none of it does, so a
// failure partway through cannot leave the user with half an import.
//
// ID and CreatedAt are filled in when unset, so a caller normally sends only
// titles and bodies.
func (s *NotesService) AddMany(ctx context.Context, notes []entity.Note) ([]entity.Note, error) {
	if len(notes) > entity.MaxNoteBatch {
		return nil, fmt.Errorf("add notes: %w: %d over the limit of %d",
			entity.ErrBatchTooLarge, len(notes), entity.MaxNoteBatch)
	}

	stored := make([]entity.Note, 0, len(notes))

	// Validated BEFORE the transaction opens. A rollback is cheap, but holding
	// the write lock while validating input is a self-inflicted contention.
	for i := range notes {
		if err := notes[i].Validate(); err != nil {
			return nil, fmt.Errorf("add notes: note %d: %w", i, err)
		}
	}

	if len(notes) == 0 {
		return stored, nil
	}

	err := s.store.Atomic(ctx, func(tx repository.DataStore) error {
		for i, note := range notes {
			saved, insertErr := tx.Notes().Insert(ctx, note)
			if insertErr != nil {
				return fmt.Errorf("note %d: %w", i, insertErr)
			}

			stored = append(stored, saved)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("add notes: %w", err)
	}

	return stored, nil
}

// List returns one page of notes, newest first.
//
// cursor is the NextCursor from the previous page, or "" for the first page. It
// is opaque by design: a client that parses it starts depending on the sort key,
// which is ours to change.
func (s *NotesService) List(
	ctx context.Context,
	cursor string,
	limit int,
) (entity.NotePage, error) {
	position, err := entity.ParseCursor(cursor)
	if err != nil {
		return entity.NotePage{}, fmt.Errorf("list notes: %w", err)
	}

	limit = entity.ClampPageSize(limit)

	// One row more than asked for is the "is there more" probe. It answers the
	// question without a second COUNT over the table, and is dropped below.
	notes, err := s.store.Notes().List(ctx, position, limit+1)
	if err != nil {
		return entity.NotePage{}, fmt.Errorf("list notes: %w", err)
	}

	page := entity.NotePage{Notes: notes}

	if len(notes) > limit {
		page.Notes = notes[:limit]

		last := page.Notes[len(page.Notes)-1]
		page.NextCursor = entity.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}.Encode()
	}

	return page, nil
}

// Get returns one note by id.
func (s *NotesService) Get(ctx context.Context, id string) (entity.Note, error) {
	note, err := s.store.Notes().GetByID(ctx, id)
	if err != nil {
		return entity.Note{}, fmt.Errorf("get note: %w", err)
	}

	return note, nil
}

// Delete removes one note.
func (s *NotesService) Delete(ctx context.Context, id string) error {
	if err := s.store.Notes().Delete(ctx, id); err != nil {
		return fmt.Errorf("delete note: %w", err)
	}

	return nil
}
