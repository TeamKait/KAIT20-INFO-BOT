package vault

import "time"

// Teacher is a parsed note from the teachers folder.
type Teacher struct {
	Name             string
	Email            string
	EmailUnconfirmed bool
	Subjects         []string
	Photo            string
	Path             string // path relative to the vault root, slash separated
	Body             string
}

// Section is a heading with its text inside a subject note.
type Section struct {
	Title string
	Text  string
}

// Subject is a parsed note from the subjects folder.
type Subject struct {
	Name     string
	Teacher  string
	Sections []Section
	Path     string
	Body     string
}

// Note is any other markdown file in the vault.
type Note struct {
	Title string
	Path  string
	Kind  string // top level folder
	Body  string
}

// Index is an immutable snapshot of the vault, swapped atomically on refresh.
type Index struct {
	Teachers []*Teacher
	Subjects []*Subject
	Notes    []*Note
	SyncedAt time.Time
}
