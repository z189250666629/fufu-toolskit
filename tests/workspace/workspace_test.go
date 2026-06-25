package workspace

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func repoPath(parts ...string) string {
	items := append([]string{"..", ".."}, parts...)
	return filepath.Clean(filepath.Join(items...))
}

func readRepoFile(t *testing.T, parts ...string) string {
	t.Helper()
	raw, err := os.ReadFile(repoPath(parts...))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestWorkspaceModules(t *testing.T) {
	for _, dir := range []string{
		"packages/go/fufu",
		"apps/fufu-tool-site",
		"apps/fufu-act",
		"apps/y2k-nav",
	} {
		t.Run(dir, func(t *testing.T) {
			fingerprintModule(t, dir)
			cmd := exec.Command("go", "test", "-count=1", "./...")
			cmd.Dir = repoPath(filepath.FromSlash(dir))
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("go test failed in %s: %v\n%s", dir, err, out)
			}
		})
	}
}

func TestLegacyModulesStayOutsideActiveWorkspace(t *testing.T) {
	goWork := readRepoFile(t, "go.work")
	for _, legacyPath := range []string{
		"legacy/network-detect",
		"apps/network-detect",
		"tools/mcy-card-upload",
	} {
		if strings.Contains(goWork, legacyPath) {
			t.Fatalf("go.work should only list active modules, but contains %q\n%s", legacyPath, goWork)
		}
	}
	for _, want := range []string{
		"legacy/network-detect/README.md",
		"tools/mcy-card-upload/README.md",
	} {
		if _, err := os.Stat(repoPath(filepath.FromSlash(want))); err != nil {
			t.Fatalf("expected archived/tooling path %s to exist: %v", want, err)
		}
	}
}

func TestFufuToolSiteDockerfileCopiesEmbeddedModules(t *testing.T) {
	dockerfile := readRepoFile(t, "apps", "fufu-tool-site", "Dockerfile")
	for _, want := range []string{
		"WORKDIR /src",
		"COPY packages/go/fufu ./packages/go/fufu",
		"COPY apps/fufu-act/go.mod ./apps/fufu-act/go.mod",
		"COPY apps/fufu-tool-site/go.mod ./apps/fufu-tool-site/go.mod",
		"WORKDIR /src/apps/fufu-tool-site",
		"COPY --from=build /out/fufu-tool-site /app/fufu-tool-site",
		"COPY apps/fufu-tool-site/web/status ./web/status",
		"COPY apps/fufu-tool-site/web/combine ./web/combine",
		"COPY apps/fufu-tool-site/config ./config",
		"COPY apps/y2k-nav/index.html ./nav/index.html",
		"COPY apps/y2k-nav/theme.mjs ./nav/theme.mjs",
		"COPY apps/y2k-nav/latency.mjs ./nav/latency.mjs",
		"COPY apps/fufu-act/public ./activity/public",
		`CMD ["/app/fufu-tool-site"]`,
	} {
		if !strings.Contains(dockerfile, want) {
			t.Fatalf("Dockerfile missing %q\n%s", want, dockerfile)
		}
	}
	for _, stale := range []string{"apps/network-detect", "legacy/network-detect", "/out/network-detect", "/app/network-detect"} {
		if strings.Contains(dockerfile, stale) {
			t.Fatalf("fufu-tool-site Dockerfile should not reference stale %q\n%s", stale, dockerfile)
		}
	}
}

func TestToolSiteUsesSharedRawConversions(t *testing.T) {
	source := readRepoFile(t, "apps", "fufu-tool-site", "newapi_client.go")
	if !strings.Contains(source, `"fufu/rawconv"`) {
		t.Fatal("fufu-tool-site should use the shared public rawconv package")
	}
	for _, duplicate := range []string{
		"func parseInt64String",
		"func jsonNumberToInt64",
	} {
		if strings.Contains(source, duplicate) {
			t.Fatalf("fufu-tool-site should not keep duplicate numeric conversion helper %q", duplicate)
		}
	}
}

func TestSharedPackagesDoNotUseInternalRawconv(t *testing.T) {
	for _, path := range []string{
		"packages/go/fufu/combine/helper_convert.go",
		"packages/go/fufu/tokens/raw.go",
	} {
		source := readRepoFile(t, filepath.FromSlash(path))
		if strings.Contains(source, `"fufu/internal/rawconv"`) {
			t.Fatalf("%s should use public fufu/rawconv instead of internal rawconv", path)
		}
	}
}

func TestBusinessAppsUseSharedStaticFileServing(t *testing.T) {
	err := filepath.WalkDir(repoPath("apps"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "node_modules", "data":
				return filepath.SkipDir
			default:
				return nil
			}
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		source := string(raw)
		for _, forbidden := range []string{"http.ServeFile", "http.FileServer"} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s should use shared fufu/webutil static serving instead of %s", path, forbidden)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func fingerprintModule(t *testing.T, dir string) {
	t.Helper()
	err := filepath.WalkDir(repoPath(filepath.FromSlash(dir)), func(path string, d fs.DirEntry, err error) error {
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
