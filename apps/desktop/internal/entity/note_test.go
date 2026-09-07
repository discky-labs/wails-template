package entity_test

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/raphaeldiscky/wails-template/apps/desktop/internal/entity"
)

func TestValidateTrimsAndAccepts(t *testing.T) {
	t.Parallel()

	note := entity.Note{Title: "  hello  ", Body: "  world  "}
	if err := note.Validate(); err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	if note.Title != "hello" || note.Body != "world" {
		t.Errorf("Validate() left Title=%q Body=%q, want trimmed", note.Title, note.Body)
	}
}

func TestValidateRejectsBlankTitle(t *testing.T) {
	t.Parallel()

	for name, title := range map[string]string{
		"empty":      "",
		"spaces":     "   ",
		"whitespace": "\t\n ",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			note := entity.Note{Title: title}
			if err := note.Validate(); !errors.Is(err, entity.ErrNoteTitleEmpty) {
				t.Errorf("Validate(%q) error = %v, want entity.ErrNoteTitleEmpty", title, err)
			}
		})
	}
}

// Length is capped in RUNES, not bytes. A byte limit would reject a title that
// is well within the limit in any language that does not fit one byte per
// character — so a title of exactly the limit in multi-byte characters must
// pass, and one character more must not.
func TestValidateCountsRunesNotBytes(t *testing.T) {
	t.Parallel()

	atLimit := entity.Note{Title: strings.Repeat("é", entity.MaxNoteTitleLen)}
	if err := atLimit.Validate(); err != nil {
		t.Errorf(
			"Validate() on a title of exactly %d runes errored: %v",
			entity.MaxNoteTitleLen,
			err,
		)
	}

	tooLong := entity.Note{Title: strings.Repeat("é", entity.MaxNoteTitleLen+1)}
	if err := tooLong.Validate(); !errors.Is(err, entity.ErrNoteTooLong) {
		t.Errorf("Validate() on an over-long title error = %v, want entity.ErrNoteTooLong", err)
	}
}

func TestValidateRejectsAnOverlongBody(t *testing.T) {
	t.Parallel()

	note := entity.Note{Title: "fine", Body: strings.Repeat("x", entity.MaxNoteBodyLen+1)}
	if err := note.Validate(); !errors.Is(err, entity.ErrNoteTooLong) {
		t.Errorf("Validate() error = %v, want entity.ErrNoteTooLong", err)
	}
}

func TestCursorRoundTrips(t *testing.T) {
	t.Parallel()

	want := entity.Cursor{
		CreatedAt: time.Date(2026, time.June, 7, 8, 9, 10, 123456789, time.UTC),
		ID:        "0f8fad5b-d9cb-469f-a165-70867728950e",
	}

	got, err := entity.ParseCursor(want.Encode())
	if err != nil {
		t.Fatalf("ParseCursor() error: %v", err)
	}

	if !got.CreatedAt.Equal(want.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, want.CreatedAt)
	}

	if got.ID != want.ID {
		t.Errorf("ID = %q, want %q", got.ID, want.ID)
	}
}

// The empty string is the first page, not an error — otherwise every caller
// would need a special case before its first call.
func TestEmptyCursorIsTheStart(t *testing.T) {
	t.Parallel()

	got, err := entity.ParseCursor("")
	if err != nil {
		t.Fatalf("ParseCursor(\"\") error: %v", err)
	}

	if !got.IsZero() {
		t.Errorf("ParseCursor(\"\") = %+v, want the zero cursor", got)
	}

	if encoded := (entity.Cursor{}).Encode(); encoded != "" {
		t.Errorf("zero Cursor.Encode() = %q, want empty", encoded)
	}
}

func TestParseCursorRejectsBadInput(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"not base64":    "!!!not-base64!!!",
		"no separator":  base64.RawURLEncoding.EncodeToString([]byte("just-one-field")),
		"empty id":      base64.RawURLEncoding.EncodeToString([]byte("2026-01-01T00:00:00Z\x00")),
		"bad timestamp": base64.RawURLEncoding.EncodeToString([]byte("not-a-time\x00some-id")),
		"reversed halves": base64.RawURLEncoding.EncodeToString(
			[]byte("some-id\x002026-01-01T00:00:00Z"),
		),
	}

	for name, encoded := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := entity.ParseCursor(encoded); !errors.Is(err, entity.ErrCursorInvalid) {
				t.Errorf("ParseCursor(%q) error = %v, want entity.ErrCursorInvalid", encoded, err)
			}
		})
	}
}

func TestClampPageSize(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		in   int
		want int
	}{
		"zero becomes the default":     {in: 0, want: entity.DefaultNotePageSize},
		"negative becomes the default": {in: -5, want: entity.DefaultNotePageSize},
		"in range is unchanged":        {in: 7, want: 7},
		"the maximum is unchanged":     {in: entity.MaxNotePageSize, want: entity.MaxNotePageSize},
		"above the maximum is capped": {
			in:   entity.MaxNotePageSize + 1,
			want: entity.MaxNotePageSize,
		},
		"absurd is capped": {in: 1_000_000, want: entity.MaxNotePageSize},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := entity.ClampPageSize(tc.in); got != tc.want {
				t.Errorf("ClampPageSize(%d) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}
