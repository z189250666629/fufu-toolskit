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

func TestY2KDockerfileCopiesSharedFufuModule(t *testing.T) {
	raw, err := os.ReadFile(filepath.FromSlash("apps/y2k-nav/Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	dockerfile := string(raw)
	for _, want := range []string{
		"WORKDIR /src",
		"COPY packages/go/fufu ./packages/go/fufu",
		"COPY apps/y2k-nav/go.mod ./apps/y2k-nav/go.mod",
		"COPY apps/y2k-nav/main.go ./apps/y2k-nav/main.go",
		"WORKDIR /src/apps/y2k-nav",
	} {
		if !strings.Contains(dockerfile, want) {
			t.Fatalf("Dockerfile missing %q\n%s", want, dockerfile)
		}
	}
}

func TestNetworkDetectUsesSharedRawConversions(t *testing.T) {
	raw, err := os.ReadFile(filepath.FromSlash("apps/network-detect/newapi_client.go"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	if !strings.Contains(source, `"fufu/rawconv"`) {
		t.Fatal("network-detect should use the shared public rawconv package")
	}
	for _, duplicate := range []string{
		"func parseInt64String",
		"func jsonNumberToInt64",
	} {
		if strings.Contains(source, duplicate) {
			t.Fatalf("network-detect should not keep duplicate numeric conversion helper %q", duplicate)
		}
	}
}

func TestSharedPackagesDoNotUseInternalRawconv(t *testing.T) {
	for _, path := range []string{
		"packages/go/fufu/combine/helper_convert.go",
		"packages/go/fufu/tokens/raw.go",
	} {
		raw, err := os.ReadFile(filepath.FromSlash(path))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), `"fufu/internal/rawconv"`) {
			t.Fatalf("%s should use public fufu/rawconv instead of internal rawconv", path)
		}
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
