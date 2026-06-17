package main

import "fmt"

func loadInitialToolConfig(root string, db *toolConfigDBStore) (ToolConfig, error) {
	seed := defaultToolConfig(root)
	cfg, ok, err := db.Load(seed)
	if err != nil {
		return ToolConfig{}, err
	}
	migratedFromFile := false
	if !ok {
		legacy := newLegacyToolConfigFile(root)
		cfg, migratedFromFile, err = legacy.Load(seed)
		if err != nil {
			return ToolConfig{}, err
		}
		if !migratedFromFile {
			cfg = seed
		}
	}

	cfg, err = normalizeToolConfig(cfg, ToolConfig{})
	if err != nil {
		return ToolConfig{}, err
	}
	if ok {
		return cfg, nil
	}
	if err := db.Save(cfg); err != nil {
		return ToolConfig{}, fmt.Errorf("seed config database: %w", err)
	}
	if migratedFromFile {
		newLegacyToolConfigFile(root).MarkMigrated()
	}
	return cfg, nil
}
