package vault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveImage(t *testing.T) {
	root := t.TempDir()
	// The real vault uses a capital К; the configured path may use lowercase.
	dir := filepath.Join(root, "Прочее", "Картинки")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	const name = "Иванов Иван.png"
	if err := os.WriteFile(filepath.Join(dir, name), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := NewService(NewSyncer("", "master", root, ""), Dirs{Images: "прочее/картинки"})

	for _, query := range []string{name, "Иванов Иван", "иванов иван.png"} {
		got, err := svc.ResolveImage(query)
		if err != nil {
			t.Errorf("ResolveImage(%q): %v", query, err)
			continue
		}
		if !strings.EqualFold(filepath.Base(got), name) {
			t.Errorf("ResolveImage(%q) = %q, want file %q", query, got, name)
		}
	}

	if _, err := svc.ResolveImage("../secret"); err == nil {
		t.Error("path traversal must be rejected")
	}
	if _, err := svc.ResolveImage("нет такой.png"); err == nil {
		t.Error("missing image must return an error")
	}
}
