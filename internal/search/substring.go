package search

import (
	"strings"

	"infobot/internal/vault"
)

// UniqueTeacherBySubstring returns the only teacher whose name contains the
// query as a case-insensitive substring. It returns nil when zero or several
// names contain it, so the caller falls back to fuzzy matching.
func UniqueTeacherBySubstring(idx *vault.Index, query string) *vault.Teacher {
	if idx == nil {
		return nil
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}

	var found *vault.Teacher
	for _, teacher := range idx.Teachers {
		if !strings.Contains(strings.ToLower(teacher.Name), q) {
			continue
		}
		if found != nil {
			return nil
		}
		found = teacher
	}
	return found
}

// UniqueSubjectBySubstring is the subject counterpart of
// UniqueTeacherBySubstring.
func UniqueSubjectBySubstring(idx *vault.Index, query string) *vault.Subject {
	if idx == nil {
		return nil
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}

	var found *vault.Subject
	for _, subject := range idx.Subjects {
		if !strings.Contains(strings.ToLower(subject.Name), q) {
			continue
		}
		if found != nil {
			return nil
		}
		found = subject
	}
	return found
}
