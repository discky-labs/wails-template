// Package entity holds the shapes that outlive a run, as they are stored.
//
// It imports no Wails package and no database driver. Both the repository
// adapters and the Wails services depend on this package, and neither depends
// on the other — so this is where the sentinel errors live too, letting a
// caller use errors.Is without reaching into the persistence layer.
package entity

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Field bounds. The frontend is untrusted input, so a note is capped before it
// reaches the database rather than after.
const (
	MaxNoteTitleLen = 120
	MaxNoteBodyLen  = 10_000
)

// Page bounds for note listings. DefaultNotePageSize applies when a caller asks
// for nothing; MaxNotePageSize is the ceiling, because a limit arriving from the
// frontend must not be able to request the whole table.
const (
	DefaultNotePageSize = 20
	MaxNotePageSize     = 100
)

// MaxNoteBatch caps a batched write. Batching is the point — one bridge
// crossing instead of N — but an unbounded batch is one transaction holding the
// write lock for as long as the caller likes.
const MaxNoteBatch = 100

// Sentinel errors. An adapter translates its driver's failures into these, so
// callers stay portable across backing stores.
var (
	// ErrNoteNotFound is returned when no note has the requested id.
	ErrNoteNotFound = errors.New("note not found")

	// ErrNoteTitleEmpty is returned when a note's title is blank.
	ErrNoteTitleEmpty = errors.New("note title must not be empty")

	// ErrNoteTooLong is returned when a note exceeds [MaxNoteTitleLen] or
	// [MaxNoteBodyLen].
	ErrNoteTooLong = errors.New("note is too long")

	// ErrCursorInvalid is returned when a pagination cursor cannot be decoded.
	// Cursors are opaque to the frontend, so a bad one is a bug or tampering,
	// never something a user typed.
	ErrCursorInvalid = errors.New("invalid cursor")

	// ErrBatchTooLarge is returned when a batched write exceeds [MaxNoteBatch].
	ErrBatchTooLarge = errors.New("batch is too large")
)

// Note is a single stored note.
type Note struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}

// Validate trims the note in place and reports the first problem found.
//
// Length is measured in runes, not bytes: a limit that counts bytes rejects a
// shorter string in any language that does not fit in one byte per character.
func (n *Note) Validate() error {
	n.Title = strings.TrimSpace(n.Title)
	n.Body = strings.TrimSpace(n.Body)

	if n.Title == "" {
		return ErrNoteTitleEmpty
	}

	if utf8.RuneCountInString(n.Title) > MaxNoteTitleLen {
		return fmt.Errorf("%w: title is over %d characters", ErrNoteTooLong, MaxNoteTitleLen)
	}

	if utf8.RuneCountInString(n.Body) > MaxNoteBodyLen {
		return fmt.Errorf("%w: body is over %d characters", ErrNoteTooLong, MaxNoteBodyLen)
	}

	return nil
}

// NotePage is one page of notes plus the cursor that continues it.
//
// NextCursor is empty on the last page, which is how a caller knows to stop —
// there is deliberately no total count, because computing one costs a second
// full scan on every page.
type NotePage struct {
	Notes      []Note `json:"notes"`
	NextCursor string `json:"nextCursor"`
}

// Cursor is a keyset position in the note listing. The zero value means "start
// at the newest".
//
// It carries the full sort key — the ordering is (CreatedAt DESC, ID DESC) —
// because a timestamp alone is not unique: a coarse system clock can stamp
// several notes identically, and the id breaks the tie so no row is skipped or
// repeated across pages.
type Cursor struct {
	CreatedAt time.Time
	ID        string
}

// cursorSep separates the two fields of an encoded cursor. NUL cannot appear in
// either half, so no escaping is needed.
const cursorSep = "\x00"

// cursorTimeLayout is the wire format for a cursor's timestamp. Only precision
// matters here — ordering is the database's job, not the cursor's.
const cursorTimeLayout = time.RFC3339Nano

// IsZero reports whether c is the start of the listing.
func (c Cursor) IsZero() bool {
	return c.ID == "" && c.CreatedAt.IsZero()
}

// Encode returns the opaque string a client hands back to fetch the next page.
//
// Base64 rather than plain text so it reads as a token: a client that starts
// parsing a cursor also starts depending on the sort key, which is ours to
// change.
func (c Cursor) Encode() string {
	if c.IsZero() {
		return ""
	}

	raw := c.CreatedAt.UTC().Format(cursorTimeLayout) + cursorSep + c.ID

	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// ParseCursor decodes a cursor produced by [Cursor.Encode]. An empty string is
// not an error — it is the first page.
func ParseCursor(encoded string) (Cursor, error) {
	if encoded == "" {
		return Cursor{}, nil
	}

	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return Cursor{}, fmt.Errorf("%w: not base64", ErrCursorInvalid)
	}

	timestamp, id, found := strings.Cut(string(raw), cursorSep)
	if !found || id == "" {
		return Cursor{}, fmt.Errorf("%w: malformed", ErrCursorInvalid)
	}

	createdAt, err := time.Parse(cursorTimeLayout, timestamp)
	if err != nil {
		return Cursor{}, fmt.Errorf("%w: bad timestamp", ErrCursorInvalid)
	}

	return Cursor{CreatedAt: createdAt, ID: id}, nil
}

// ClampPageSize brings a caller-supplied limit into range.
//
// It clamps rather than rejects: a limit is a hint from the UI, and failing a
// listing because a caller asked for 1000 rows helps nobody.
//
// This is the API-level ceiling, applied by the service — the trust boundary.
// The repository does not apply it again: a page is fetched with limit+1 as a
// "is there more" probe, and a second clamp would silently swallow that extra
// row at the maximum page size and end pagination one page early.
func ClampPageSize(limit int) int {
	if limit <= 0 {
		return DefaultNotePageSize
	}

	if limit > MaxNotePageSize {
		return MaxNotePageSize
	}

	return limit
}
