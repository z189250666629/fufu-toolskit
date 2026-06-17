package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type legacyToolConfigFile struct {
	path string
}

func newLegacyToolConfigFile(root string) legacyToolConfigFile {
	return legacyToolConfigFile{path: filepath.Join(root, "data", toolConfigFileName)}
}

func (f legacyToolConfigFile) Load(seed ToolConfig) (ToolConfig, bool, error) {
	raw, err := os.ReadFile(f.path)
	if errors.Is(err, os.ErrNotExist) {
		return ToolConfig{}, false, nil
	}
	if err != nil {
		return ToolConfig{}, false, fmt.Errorf("%s 读取失败: %w", toolConfigFileName, err)
	}
	cfg, err := decodeStoredToolConfig(raw, seed, toolConfigFileName)
	if err != nil {
		return ToolConfig{}, false, err
	}
	return cfg, true, nil
}

func (f legacyToolConfigFile) MarkMigrated() {
	// Best effort: the database has already been seeded, so a rename failure
	// should not block boot. A remaining file will be ignored after DB exists.
	_ = os.Rename(f.path, f.path+".migrated")
}
