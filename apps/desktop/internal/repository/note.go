package repository

import (
	"context"

	"github.com/raphaeldiscky/wails-template/apps/desktop/internal/entity"
)

// NoteRepository is the persistence contract for notes.
//
// An implementation MUST translate its driver's failures into the entity
// package's sentinels — entity.ErrNoteNotFound above all — so that callers
// match on a domain error and never learn which database is underneath.
//
// Four methods, and it stays small on purpose: past roughly five, split by role
// into a reader and a writer rather than growing one interface that every
// caller then depends on in full.
type NoteRepository interface {
	// Insert stores n, filling in ID and CreatedAt when they are unset, and
	// returns the note as stored.
	Insert(ctx context.Context, n entity.Note) (entity.Note, error)

	// GetByID returns one note, or entity.ErrNoteNotFound.
	GetByID(ctx context.Context, id string) (entity.Note, error)

	// List returns up to limit notes, newest first, resuming after cursor. A
	// zero cursor starts at the newest. A limit below 1 becomes
	// entity.DefaultNotePageSize; the UPPER bound is the caller's to enforce,
	// via entity.ClampPageSize.
	List(ctx context.Context, cursor entity.Cursor, limit int) ([]entity.Note, error)

	// Delete removes one note, or returns entity.ErrNoteNotFound when there was
	// nothing to remove.
	Delete(ctx context.Context, id string) error
}
