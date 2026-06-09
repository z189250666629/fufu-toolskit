package workspace

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceModules(t *testing.T) {
	for _, dir := range []string{
		"packages/go/fufu",
		"apps/network-detect",
		"apps/fufu-act",
		"apps/y2k-nav",
	} {
		t.Run(dir, func(t *testing.T) {
			fingerprintModule(t, dir)
			cmd := exec.Command("go", "test", "-count=1", "./...")
			cmd.Dir = filepath.FromSlash(dir)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("go test failed in %s: %v\n%s", dir, err, out)
			}
		})
	}
}

func fingerprintModule(t *testing.T, dir string) {
	t.Helper()
	err := filepath.WalkDir(filepath.FromSlash(dir), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "data", "node_modules":
				return filepath.SkipDir
			default:
				return nil
			}
		}
		name := d.Name()
		if strings.HasSuffix(name, ".go") || name == "go.mod" || name == "go.sum" || name == "package.json" {
			_, err := os.ReadFile(path)
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("fingerprint %s: %v", dir, err)
	}
}
