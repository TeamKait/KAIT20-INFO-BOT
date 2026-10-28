package vault

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestSyncClone is an optional network test: it clones the real vault repo and
// checks that the second sync is a no-op.
func TestSyncClone(t *testing.T) {
	if os.Getenv("RUN_GIT_TEST") == "" {
		t.Skip("set RUN_GIT_TEST=1 to run the network git test")
	}

	dir := filepath.Join(t.TempDir(), "vault")
	syncer := NewSyncer("https://github.com/TeamKait/KAIT20-INFO.git", "master", dir, "")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	updated, err := syncer.Sync(ctx)
	if err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if !updated {
		t.Fatal("first sync must clone the repository")
	}
	if !HasNotes(dir) {
		t.Fatal("cloned vault has no markdown files")
	}

	updated, err = syncer.Sync(ctx)
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if updated {
		t.Error("second sync reports changes, want no-op")
	}
}
