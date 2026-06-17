package main

import (
	"errors"
	"fufu/activity"
)

func (s *toolConfigStore) SavePatch(patch adminConfigPatch) (ToolConfig, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return ToolConfig{}, false, errors.New("config database is not initialized")
	}
	normalized, newAPIChanged, err := applyAdminConfigPatch(s.cfg, patch)
	if err != nil {
		return ToolConfig{}, false, err
	}
	if err := s.db.Save(normalized); err != nil {
		return ToolConfig{}, false, err
	}
	s.cfg = normalized
	return cloneToolConfig(normalized), newAPIChanged, nil
}

func applyAdminConfigPatch(current ToolConfig, patch adminConfigPatch) (ToolConfig, bool, error) {
	next := cloneToolConfig(current)
	newAPIChanged := false
	if patch.NewAPI != nil {
		sites, err := normalizeManagedAPISiteConfigs(patch.NewAPI.Sites, next.NewAPI.Sites)
		if err != nil {
			return ToolConfig{}, false, err
		}
		next.NewAPI.Sites = sites
		newAPIChanged = true
	}
	if patch.Navigation != nil {
		next.Navigation = *patch.Navigation
	}
	if patch.Activity != nil {
		next.Activity = activity.CloneConfig(*patch.Activity)
	}
	if patch.MCY != nil {
		next.MCY = *patch.MCY
	}
	normalized, err := normalizeToolConfig(next, current)
	if err != nil {
		return ToolConfig{}, false, err
	}
	return normalized, newAPIChanged, nil
}
