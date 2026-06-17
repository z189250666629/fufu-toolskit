package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

type toolConfigDBStore struct {
	db *sql.DB
}

func openToolConfigDBStore(path string) (*toolConfigDBStore, error) {
	db, err := openToolConfigDB(path)
	if err != nil {
		return nil, err
	}
	return &toolConfigDBStore{db: db}, nil
}

func (s *toolConfigDBStore) Load(seed ToolConfig) (ToolConfig, bool, error) {
	raw, ok, err := readToolConfigRow(s.db)
	if err != nil {
		return ToolConfig{}, false, fmt.Errorf("read config database: %w", err)
	}
	if !ok {
		return ToolConfig{}, false, nil
	}
	cfg, err := decodeStoredToolConfig(raw, seed, toolConfigDBName)
	if err != nil {
		return ToolConfig{}, false, err
	}
	return cfg, true, nil
}

func (s *toolConfigDBStore) Save(cfg ToolConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return writeToolConfigRow(s.db, data)
}

func (s *toolConfigDBStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

func decodeStoredToolConfig(raw []byte, seed ToolConfig, source string) (ToolConfig, error) {
	cfg := cloneToolConfig(seed)
	// Stored JSON owns the NewAPI site list; default/env sites only seed fresh
	// boots and must not leak into decoded database or legacy snapshots.
	cfg.NewAPI.Sites = nil
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return ToolConfig{}, fmt.Errorf("%s 不是有效 JSON: %w", source, err)
	}
	return cfg, nil
}
