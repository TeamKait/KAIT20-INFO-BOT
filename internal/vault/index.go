package vault

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Dirs names the folders that hold structured notes. Empty values fall back
// to the names used by the Sharaga vault.
type Dirs struct {
	Teachers string
	Subjects string
}

func (d Dirs) teachers() string {
	if d.Teachers == "" {
		return "Преподаватели"
	}
	return d.Teachers
}

func (d Dirs) subjects() string {
	if d.Subjects == "" {
		return "Предметы"
	}
	return d.Subjects
}

// BuildIndex walks the vault and parses every markdown file.
func BuildIndex(root string, dirs Dirs) (*Index, error) {
	root = filepath.Clean(root)
	idx := &Index{}

	err := filepath.WalkDir(root, func(p string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Skip unreadable entries instead of failing the whole refresh.
			return nil
		}
		if entry.IsDir() {
			if p == root {
				return nil
			}
			if skipDir(entry.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(p), ".md") {
			return nil
		}

		data, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		title := strings.TrimSpace(strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())))
		top := ""
		if i := strings.Index(rel, "/"); i >= 0 {
			top = rel[:i]
		}

		switch top {
		case dirs.teachers():
			idx.Teachers = append(idx.Teachers, parseTeacher(title, rel, data))
		case dirs.subjects():
			idx.Subjects = append(idx.Subjects, parseSubject(title, rel, data))
		default:
			idx.Notes = append(idx.Notes, parseNote(title, rel, top, data))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(idx.Teachers, func(i, j int) bool { return idx.Teachers[i].Name < idx.Teachers[j].Name })
	sort.Slice(idx.Subjects, func(i, j int) bool { return idx.Subjects[i].Name < idx.Subjects[j].Name })
	sort.Slice(idx.Notes, func(i, j int) bool { return idx.Notes[i].Path < idx.Notes[j].Path })

	return idx, nil
}

// HasNotes reports whether the vault directory already contains markdown.
func HasNotes(root string) bool {
	found := false
	_ = filepath.WalkDir(root, func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if p == root {
				return nil
			}
			if skipDir(entry.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(p), ".md") {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found
}

func skipDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch strings.ToLower(name) {
	case "шаблоны", "templates":
		return true
	}
	return false
}
