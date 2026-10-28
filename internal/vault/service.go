package vault

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Service owns the git clone and the current index snapshot.
type Service struct {
	syncer *Syncer
	dirs   Dirs
	mu     sync.Mutex
	index  atomic.Pointer[Index]
}

func NewService(syncer *Syncer, dirs Dirs) *Service {
	return &Service{syncer: syncer, dirs: dirs}
}

// Index returns the latest snapshot. It is never mutated after publication.
func (s *Service) Index() *Index {
	return s.index.Load()
}

// Refresh pulls the repository (if needed) and rebuilds the index.
func (s *Service) Refresh(ctx context.Context) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updated, err := s.syncer.Sync(ctx)
	if err != nil {
		return false, err
	}
	if err := s.rebuild(); err != nil {
		return false, err
	}
	return updated, nil
}

// LoadIndex rebuilds the index from the files already on disk, without
// touching git. Used when the network is down but a clone exists.
func (s *Service) LoadIndex() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rebuild()
}

func (s *Service) rebuild() error {
	idx, err := BuildIndex(s.syncer.Dir(), s.dirs)
	if err != nil {
		return err
	}
	idx.SyncedAt = time.Now()
	s.index.Store(idx)
	return nil
}
