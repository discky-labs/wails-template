// Package repository is the persistence layer: it owns every SQL statement in
// this application, and nothing outside it may contain one.
//
// # What this layer may import
//
// The entity package, the standard library, and its driver. That is all.
//
// # What this layer may NOT import
//
//   - Any Wails package. A repository is plain Go, tested against a temp file
//     with no webview in sight.
//   - Any service. Dependencies point one way — services reach persistence
//     through [DataStore]; persistence never reaches back.
//
// # Where things go
//
//   - <entity>.go holds the PORT, an interface named <Entity>Repository.
//   - <entity>_sqlite.go holds the ADAPTER and its SQL, as package consts.
//     A second backing store would be <entity>_pg.go beside it, satisfying the
//     same port — which is what the suffix is for.
//   - migrations/ holds the schema, one file per table.
//
// The adapter translates its driver's errors into the entity package's
// sentinels, so a caller matches on entity.ErrNoteNotFound and never learns
// which database it is talking to.
//
// # The one port here, and why it is not speculation
//
// A repository interface earns its keep even with a single implementation: it
// is the seam [SQLite.Atomic] needs to hand a transaction-scoped store to a
// caller, and the seam a test needs to substitute a fake. That is the standard
// this repo holds ports to — see .claude/rules/go/persistence.md. Preferences
// deliberately have no port, because they have neither need.
package repository
