package main

import (
	"fufu/newapi"
	"sync"
)

type toolConfigStore struct {
	mu   sync.RWMutex
	path string
	db   *toolConfigDBStore
	cfg  ToolConfig
}

func newToolConfigStore(path string) *toolConfigStore {
	return &toolConfigStore{path: path}
}

// Load opens the SQLite config database and resolves the active configuration.
// The database is the source of truth: once seeded, environment variables are
// ignored. On first boot the store seeds itself from the legacy tool-config.json
// (migrating existing deployments) or, failing that, from environment defaults,
// then persists the result so future redeploys no longer depend on env.
func (s *toolConfigStore) Load(root string) error {
	db, err := openToolConfigDBStore(s.path)
	if err != nil {
		return err
	}
	cfg, err := loadInitialToolConfig(root, db)
	if err != nil {
		_ = db.Close()
		return err
	}

	s.mu.Lock()
	s.db = db
	s.cfg = cfg
	s.mu.Unlock()
	applyToolConfigSnapshot(cfg)
	return nil
}

// Close releases the underlying database handle.
func (s *toolConfigStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

func (s *toolConfigStore) Snapshot() ToolConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneToolConfig(s.cfg)
}

func (s *toolConfigStore) ManagedSites() []newapi.Site {
	cfg := s.Snapshot()
	sites := make([]newapi.Site, 0, len(cfg.NewAPI.Sites))
	for _, site := range cfg.NewAPI.Sites {
		sites = append(sites, newAPISitesFromManagedConfig(site)...)
	}
	return sites
}
