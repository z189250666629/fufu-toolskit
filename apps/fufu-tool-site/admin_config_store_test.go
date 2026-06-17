package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestToolConfigDBStoreLoadsStoredConfigWithoutSeedSiteLeak(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", toolConfigDBName)
	db, err := openToolConfigDBStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	seed := ToolConfig{
		NewAPI: NewAPIAdminConfig{Sites: []ManagedAPISiteConfig{{
			Name:  "seed-site",
			URL:   "https://seed.example.test",
			Token: "seed-token",
		}}},
		Navigation: defaultNavigationConfig(),
	}
	stored := ToolConfig{
		NewAPI: NewAPIAdminConfig{Sites: []ManagedAPISiteConfig{{
			Name:  "stored-site",
			URL:   "https://stored.example.test",
			Token: "stored-token",
		}}},
		Navigation: seed.Navigation,
	}

	if err := db.Save(stored); err != nil {
		t.Fatal(err)
	}
	got, ok, err := db.Load(seed)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected stored config row")
	}
	if len(got.NewAPI.Sites) != 1 || got.NewAPI.Sites[0].Name != "stored-site" {
		t.Fatalf("stored sites should replace seed sites, got %#v", got.NewAPI.Sites)
	}
}

func TestLegacyToolConfigFileLoadsAndMarksMigrated(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}
	legacyPath := filepath.Join(dataDir, toolConfigFileName)
	if err := os.WriteFile(legacyPath, []byte(`{"newapi":{"sites":[{"name":"legacy-site","url":"https://legacy.example.test","token":"legacy-token"}]}}`), 0600); err != nil {
		t.Fatal(err)
	}

	legacy := newLegacyToolConfigFile(root)
	got, ok, err := legacy.Load(ToolConfig{
		NewAPI: NewAPIAdminConfig{Sites: []ManagedAPISiteConfig{{
			Name:  "seed-site",
			URL:   "https://seed.example.test",
			Token: "seed-token",
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected legacy config file")
	}
	if len(got.NewAPI.Sites) != 1 || got.NewAPI.Sites[0].Name != "legacy-site" {
		t.Fatalf("legacy sites should replace seed sites, got %#v", got.NewAPI.Sites)
	}

	legacy.MarkMigrated()
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("legacy file should be renamed, stat err=%v", err)
	}
	if _, err := os.Stat(legacyPath + ".migrated"); err != nil {
		t.Fatalf("expected migrated backup: %v", err)
	}
}

func TestApplyAdminConfigPatchNormalizesBeforeSave(t *testing.T) {
	current := ToolConfig{
		NewAPI: NewAPIAdminConfig{Sites: []ManagedAPISiteConfig{{
			Name:     "api-site",
			Category: "api",
			URL:      "https://api-1.example.test",
			Token:    "old-token",
		}}},
		MCY: MCYAdminConfig{Password: "old-password"},
	}
	patch := adminConfigPatch{
		NewAPI: &struct {
			Sites []ManagedAPISiteConfig `json:"sites"`
		}{Sites: []ManagedAPISiteConfig{{
			Name:     "api-site",
			Category: "api",
			URL:      "https://api-2.example.test/",
		}}},
		MCY: &MCYAdminConfig{
			BaseURL:  "http://shop.example.test/admin",
			Username: " shop-user ",
		},
	}

	got, newAPIChanged, err := applyAdminConfigPatch(current, patch)
	if err != nil {
		t.Fatal(err)
	}
	if !newAPIChanged {
		t.Fatal("expected NewAPI change marker")
	}
	if got.NewAPI.Sites[0].Token != "old-token" || got.NewAPI.Sites[0].URL != "https://api-2.example.test" {
		t.Fatalf("NewAPI site should inherit token and normalize URL, got %#v", got.NewAPI.Sites[0])
	}
	if got.MCY.BaseURL != "https://shop.example.test" || got.MCY.Username != "shop-user" || got.MCY.Password != "old-password" {
		t.Fatalf("MCY config should be normalized and inherit password, got %#v", got.MCY)
	}
	if current.NewAPI.Sites[0].URL != "https://api-1.example.test" {
		t.Fatalf("patch must not mutate current snapshot, got %#v", current.NewAPI.Sites[0])
	}
}
