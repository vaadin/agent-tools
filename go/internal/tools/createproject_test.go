package tools

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/vaadin/agent-tools/internal/tool"
)

func TestSanitizeName(t *testing.T) {
	cases := map[string]string{
		"my-vaadin-app": "my-vaadin-app",
		"  My App!  ":   "MyApp",
		"foo/bar.baz":   "foobarbaz",
		"a_b-c":         "a_b-c",
		"!!!":           "",
	}
	for in, want := range cases {
		if got := sanitizeName(in); got != want {
			t.Errorf("sanitizeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildSkeletonURL(t *testing.T) {
	cases := []struct {
		name    string
		example string
		pre     bool
		want    string
	}{
		{"my-app", "flow", false, "https://start.vaadin.com/skeleton?artifactId=my-app&frameworks=flow&ref=cli"},
		{"my-app", "none", false, "https://start.vaadin.com/skeleton?artifactId=my-app&ref=cli"},
		{"my-app", "flow", true, "https://start.vaadin.com/skeleton?artifactId=my-app&frameworks=flow&platformVersion=pre&ref=cli"},
	}
	for _, c := range cases {
		if got := buildSkeletonURL(c.name, c.example, c.pre); got != c.want {
			t.Errorf("buildSkeletonURL(%q, %q, %v) = %q, want %q", c.name, c.example, c.pre, got, c.want)
		}
	}
}

func TestStripFirst(t *testing.T) {
	cases := map[string]string{
		"proj/pom.xml":           "pom.xml",
		"proj/src/main/App.java": "src/main/App.java",
		"proj/":                  "",
		"toplevel":               "",
		"./proj/pom.xml":         "pom.xml",
	}
	for in, want := range cases {
		if got := stripFirst(in); got != want {
			t.Errorf("stripFirst(%q) = %q, want %q", in, got, want)
		}
	}
}

// makeZip builds an in-memory zip from name->content. A name ending in "/" is a
// directory entry; an empty content string yields a zero-length file.
func makeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		if len(name) > 0 && name[len(name)-1] == '/' {
			if _, err := zw.Create(name); err != nil {
				t.Fatal(err)
			}
			continue
		}
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestExtractZipStripsTopDirAndKeepsEmptyFiles(t *testing.T) {
	data := makeZip(t, map[string]string{
		"my-project/":                             "",
		"my-project/pom.xml":                      "<project/>",
		"my-project/src/main/java/App.java":       "class App {}",
		"my-project/frontend/themes/x/styles.css": "", // empty theme file must survive
	})

	dir := t.TempDir()
	count, err := extractZip(data, dir)
	if err != nil {
		t.Fatalf("extractZip: %v", err)
	}
	if count != 3 {
		t.Fatalf("files written = %d, want 3", count)
	}
	// Top-level "my-project" directory was stripped.
	if _, err := os.Stat(filepath.Join(dir, "my-project")); !os.IsNotExist(err) {
		t.Error("expected top-level directory to be stripped")
	}
	// Files landed at the stripped paths.
	for _, p := range []string{"pom.xml", "src/main/java/App.java", "frontend/themes/x/styles.css"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(p))); err != nil {
			t.Errorf("expected %s to exist: %v", p, err)
		}
	}
	// The empty styles.css must exist and be zero length.
	info, err := os.Stat(filepath.Join(dir, "frontend/themes/x/styles.css"))
	if err != nil {
		t.Fatalf("styles.css missing: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("styles.css size = %d, want 0", info.Size())
	}
}

func TestExtractZipRejectsZipSlip(t *testing.T) {
	data := makeZip(t, map[string]string{
		"proj/../../escape.txt": "pwned",
	})
	dir := t.TempDir()
	if _, err := extractZip(data, dir); err == nil {
		t.Fatal("expected zip-slip path to be rejected")
	}
}

func TestCreateProjectUsageErrors(t *testing.T) {
	// No target directory.
	if r := createProject(tool.Args{Cwd: t.TempDir()}); r.UsageError == "" {
		t.Error("expected usage error when target directory is missing")
	}

	// Existing, non-empty target without --overwrite (checked before any network).
	dir := t.TempDir()
	existing := filepath.Join(dir, "proj")
	if err := os.MkdirAll(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(existing, "keep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := createProject(tool.Args{Positionals: []string{"proj"}, Cwd: dir}); r.UsageError == "" {
		t.Error("expected usage error for a non-empty target without --overwrite")
	}

	// Invalid --example.
	if r := createProject(tool.Args{Positionals: []string{"newproj", "--example=react"}, Cwd: t.TempDir()}); r.UsageError == "" {
		t.Error("expected usage error for an invalid --example value")
	}
}
