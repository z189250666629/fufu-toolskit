package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func modelStatusCacheKey(root string) string {
	hash := sha256.New()
	writeCachePart(hash, "root", root)
	for _, name := range managedSiteCacheEnvNames() {
		writeCachePart(hash, name, os.Getenv(name))
	}
	for _, path := range managedSiteCacheFileCandidates(root) {
		writeCachePart(hash, "file", managedSiteCacheFileState(path))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

type cacheHash interface {
	Write([]byte) (int, error)
}

func writeCachePart(hash cacheHash, name, value string) {
	_, _ = hash.Write([]byte(name))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(value))
	_, _ = hash.Write([]byte{0})
}

func managedSiteCacheEnvNames() []string {
	names := []string{
		"NEWAPI_MANAGED_API_SITES",
		"NEWAPI_MANAGED_API_CONFIG",
		"NEWAPI_API_SITE_URL",
		"NEWAPI_API_SITE_TOKEN",
		"NEWAPI_API_SITE_ACCESS_TOKEN",
		"NEWAPI_TOKEN_SITE_URL",
		"NEWAPI_TOKEN_SITE_TOKEN",
		"NEWAPI_TOKEN_SITE_ACCESS_TOKEN",
	}
	for i := 1; i <= 10; i++ {
		prefix := fmt.Sprintf("NEWAPI_MANAGED_SITE_%d", i)
		for _, suffix := range []string{
			"_NAME",
			"_URL",
			"_TOKEN",
			"_ACCESS_TOKEN",
			"_USER_ID",
			"_KIND",
			"_QUOTA_UNIT",
			"_CURRENCY",
			"_RECHARGE_RATIO",
			"_CHANNEL_LIST_ENDPOINT",
			"_SKIP_USER_HEADER",
			"_NOTE",
		} {
			names = append(names, prefix+suffix)
		}
	}
	return names
}

func managedSiteCacheFileCandidates(root string) []string {
	configured := strings.TrimSpace(os.Getenv("NEWAPI_MANAGED_API_CONFIG"))
	if configured != "" {
		return []string{resolveManagedSiteConfigPath(root, configured)}
	}
	candidates := []string{filepath.Join(root, "newapi-managed-api-sites.json")}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, "Downloads", "newapi-manager-config-2026-05-06.json"))
	}
	return candidates
}

func resolveManagedSiteConfigPath(root, path string) string {
	if path == "" || filepath.IsAbs(path) || root == "" {
		return path
	}
	return filepath.Join(root, path)
}

func managedSiteCacheFileState(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return path + "|missing"
	}
	return fmt.Sprintf("%s|%d|%d", path, info.Size(), info.ModTime().UnixNano())
}
