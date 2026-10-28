package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveImage finds an attachment referenced as ![[name]] in the vault.
// Obsidian keeps attachments in a single folder; matching is case-insensitive
// because the vault is authored on Windows but the bot runs on Linux.
func (s *Service) ResolveImage(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("empty image name")
	}

	dir, err := s.imagesDir()
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(dir, filepath.FromSlash(name))
	if !withinDir(dir, candidate) {
		return "", fmt.Errorf("image %q escapes %s", name, s.dirs.images())
	}
	if isRegularFile(candidate) {
		return candidate, nil
	}

	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		for _, suffix := range []string{".png", ".jpg", ".jpeg", ".webp"} {
			if path := candidate + suffix; isRegularFile(path) {
				return path, nil
			}
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("images dir %s: %w", s.dirs.images(), err)
	}

	want := strings.ToLower(name)
	wantStem := strings.TrimSuffix(want, ext)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		got := strings.ToLower(entry.Name())
		if got == want {
			return filepath.Join(dir, entry.Name()), nil
		}
		if ext == "" && strings.TrimSuffix(got, strings.ToLower(filepath.Ext(got))) == wantStem {
			return filepath.Join(dir, entry.Name()), nil
		}
	}

	return "", fmt.Errorf("image %q not found in %s", name, s.dirs.images())
}

// imagesDir resolves the configured attachments folder segment by segment, so
// IMAGES_DIR=Прочее/картинки also works when the checkout is Прочее/Картинки
// (Linux file systems are case-sensitive, unlike the author's Windows).
func (s *Service) imagesDir() (string, error) {
	rel := s.dirs.images()
	if filepath.IsAbs(rel) {
		if info, err := os.Stat(rel); err == nil && info.IsDir() {
			return rel, nil
		}
		return "", fmt.Errorf("images dir %q not found", rel)
	}

	current := s.syncer.Dir()
	for _, segment := range strings.Split(filepath.ToSlash(rel), "/") {
		if segment == "" || segment == "." {
			continue
		}

		candidate := filepath.Join(current, segment)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			current = candidate
			continue
		}

		entries, err := os.ReadDir(current)
		if err != nil {
			return "", fmt.Errorf("images dir %q: %w", rel, err)
		}

		found := ""
		for _, entry := range entries {
			if entry.IsDir() && strings.EqualFold(entry.Name(), segment) {
				found = entry.Name()
				break
			}
		}
		if found == "" {
			return "", fmt.Errorf("images dir %q not found", rel)
		}
		current = filepath.Join(current, found)
	}
	return current, nil
}

func withinDir(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel != ".." &&
		!strings.HasPrefix(rel, ".."+string(filepath.Separator)) &&
		!filepath.IsAbs(rel)
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
