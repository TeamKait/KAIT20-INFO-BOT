package vault

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

// Syncer keeps a local clone of the vault repository up to date.
type Syncer struct {
	url    string
	branch string
	dir    string
	token  string
}

func NewSyncer(url, branch, dir, token string) *Syncer {
	if strings.TrimSpace(branch) == "" {
		branch = "master"
	}
	return &Syncer{url: url, branch: branch, dir: dir, token: token}
}

func (s *Syncer) Dir() string { return s.dir }

// IsGitRepo reports whether the vault directory is a git checkout managed by
// the bot. A plain directory is treated as a static vault (useful locally).
func (s *Syncer) IsGitRepo() bool {
	_, err := os.Stat(filepath.Join(s.dir, ".git"))
	return err == nil
}

// Sync clones the repository on first run and hard-resets it to origin/<branch>
// afterwards. The vault is read-only for the bot, so a hard reset is safe and
// never produces merge conflicts.
func (s *Syncer) Sync(ctx context.Context) (bool, error) {
	if s.IsGitRepo() {
		return s.pull(ctx)
	}
	if hasMarkdown(s.dir) {
		return false, nil
	}
	if err := s.clone(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Syncer) auth() *githttp.BasicAuth {
	if s.token == "" {
		return nil
	}
	return &githttp.BasicAuth{Username: "x-access-token", Password: s.token}
}

func (s *Syncer) clone(ctx context.Context) error {
	if err := os.MkdirAll(filepath.Dir(s.dir), 0o755); err != nil {
		return fmt.Errorf("create vault parent: %w", err)
	}

	tmp := s.dir + ".clone"
	_ = os.RemoveAll(tmp)

	_, err := git.PlainCloneContext(ctx, tmp, false, &git.CloneOptions{
		URL:           s.url,
		ReferenceName: plumbing.NewBranchReferenceName(s.branch),
		SingleBranch:  true,
		Auth:          s.auth(),
	})
	if err != nil {
		_ = os.RemoveAll(tmp)
		return fmt.Errorf("clone %s: %w", redactURL(s.url), err)
	}

	// An empty leftover directory would block the rename.
	if entries, err := os.ReadDir(s.dir); err == nil && len(entries) == 0 {
		_ = os.Remove(s.dir)
	}
	if err := os.Rename(tmp, s.dir); err != nil {
		_ = os.RemoveAll(tmp)
		return fmt.Errorf("move clone into place: %w", err)
	}
	return nil
}

func (s *Syncer) pull(ctx context.Context) (bool, error) {
	repo, err := git.PlainOpen(s.dir)
	if err != nil {
		return false, fmt.Errorf("open vault repo: %w", err)
	}

	remote, err := repo.Remote("origin")
	if err != nil {
		return false, fmt.Errorf("vault remote: %w", err)
	}

	refSpec := config.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", s.branch, s.branch))
	err = remote.FetchContext(ctx, &git.FetchOptions{
		RefSpecs: []config.RefSpec{refSpec},
		Auth:     s.auth(),
		Force:    true,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return false, fmt.Errorf("fetch origin/%s: %w", s.branch, err)
	}

	remoteRef, err := repo.Reference(plumbing.NewRemoteReferenceName("origin", s.branch), true)
	if err != nil {
		return false, fmt.Errorf("resolve origin/%s: %w", s.branch, err)
	}

	head, err := repo.Head()
	if err != nil {
		return false, fmt.Errorf("read HEAD: %w", err)
	}
	if head.Hash() == remoteRef.Hash() {
		return false, nil
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("vault worktree: %w", err)
	}
	if err := worktree.Reset(&git.ResetOptions{
		Commit: remoteRef.Hash(),
		Mode:   git.HardReset,
	}); err != nil {
		return false, fmt.Errorf("reset vault to origin/%s: %w", s.branch, err)
	}
	return true, nil
}

func hasMarkdown(dir string) bool {
	return HasNotes(dir)
}

// redactURL hides credentials in log messages.
func redactURL(raw string) string {
	if i := strings.Index(raw, "@"); i >= 0 && strings.Contains(raw[:i], "://") {
		scheme := raw[:strings.Index(raw, "://")+3]
		return scheme + "***@" + raw[i+1:]
	}
	return raw
}
