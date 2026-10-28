package search

import (
	"sort"

	"infobot/internal/vault"
)

type TeacherMatch struct {
	Teacher *vault.Teacher
	Score   float64
}

type SubjectMatch struct {
	Subject *vault.Subject
	Score   float64
}

// DocMatch is a note of any kind: teacher, subject or a regular note.
type DocMatch struct {
	Title string
	Path  string
	Kind  string
	Body  string
	Score float64
}

// Teachers ranks teachers by how well their name matches the query.
func Teachers(idx *vault.Index, query string) []TeacherMatch {
	if idx == nil {
		return nil
	}
	q := Normalize(query)
	if q == "" {
		return nil
	}

	out := make([]TeacherMatch, 0, len(idx.Teachers))
	for _, t := range idx.Teachers {
		out = append(out, TeacherMatch{Teacher: t, Score: matchScore(q, Normalize(t.Name))})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Teacher.Name < out[j].Teacher.Name
	})
	return out
}

// Subjects ranks subjects by name similarity.
func Subjects(idx *vault.Index, query string) []SubjectMatch {
	if idx == nil {
		return nil
	}
	q := Normalize(query)
	if q == "" {
		return nil
	}

	out := make([]SubjectMatch, 0, len(idx.Subjects))
	for _, s := range idx.Subjects {
		out = append(out, SubjectMatch{Subject: s, Score: matchScore(q, Normalize(s.Name))})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Subject.Name < out[j].Subject.Name
	})
	return out
}

// Documents searches every markdown note by title and falls back to full text
// search, so "/find аттестация" still works when no note is named that way.
func Documents(idx *vault.Index, query string) []DocMatch {
	if idx == nil {
		return nil
	}
	q := Normalize(query)
	tokens := Tokens(q)
	if len(tokens) == 0 {
		return nil
	}

	var out []DocMatch
	add := func(title, path, kind, body string) {
		score := matchScore(q, Normalize(title))
		if score < 0.55 {
			if !containsAll(Normalize(body), tokens) {
				return
			}
			score = 0.4
		}
		out = append(out, DocMatch{Title: title, Path: path, Kind: kind, Body: body, Score: score})
	}

	for _, t := range idx.Teachers {
		add(t.Name, t.Path, "Преподаватель", t.Body)
	}
	for _, s := range idx.Subjects {
		add(s.Name, s.Path, "Предмет", s.Body)
	}
	for _, n := range idx.Notes {
		add(n.Title, n.Path, n.Kind, n.Body)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Title < out[j].Title
	})
	return out
}
