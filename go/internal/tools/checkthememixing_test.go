package tools

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/vaadin/agent-tools/internal/lib"
	"github.com/vaadin/agent-tools/internal/tool"
)

// fixturesDir resolves the shared test/fixtures directory at the repo root,
// reused verbatim from the JavaScript test suite (../../../test/fixtures
// relative to this file: go/internal/tools -> repo root -> test/fixtures).
func fixturesDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file path")
	}
	repoRoot := filepath.Join(filepath.Dir(file), "..", "..", "..")
	return filepath.Join(repoRoot, "test", "fixtures")
}

func run(t *testing.T, fixture string) themeMixingReport {
	t.Helper()
	return analyzeThemeMixing(tool.Args{Positionals: []string{fixture}, Cwd: fixturesDir(t)})
}

func findByCode(findings []lib.Finding, code string) *lib.Finding {
	for i := range findings {
		if findings[i].Code == code {
			return &findings[i]
		}
	}
	return nil
}

func TestFlagsBothAuraAndLumo(t *testing.T) {
	r := run(t, "mixed")
	if r.OK {
		t.Fatal("expected ok=false")
	}
	if got := strings.Join(r.ThemesLoaded, ","); got != "aura,lumo" {
		t.Fatalf("themesLoaded = %q, want aura,lumo", got)
	}
	f := findByCode(r.Findings, "MULTIPLE_BASE_THEMES")
	if f == nil {
		t.Fatal("expected MULTIPLE_BASE_THEMES finding")
	}
	if f.Level != "error" {
		t.Fatalf("level = %q, want error", f.Level)
	}
}

func TestDetectsFullyQualifiedAnnotations(t *testing.T) {
	r := run(t, "qualified")
	if r.OK {
		t.Fatal("expected ok=false for fully qualified Aura + Lumo")
	}
	if got := strings.Join(r.ThemesLoaded, ","); got != "aura,lumo" {
		t.Fatalf("themesLoaded = %q, want aura,lumo", got)
	}
	if findByCode(r.Findings, "MULTIPLE_BASE_THEMES") == nil {
		t.Fatal("expected MULTIPLE_BASE_THEMES finding from qualified annotations")
	}
}

func TestPassesSingleTheme(t *testing.T) {
	r := run(t, "clean")
	if !r.OK {
		t.Fatal("expected ok=true")
	}
	if got := strings.Join(r.ThemesLoaded, ","); got != "aura" {
		t.Fatalf("themesLoaded = %q, want aura", got)
	}
	if len(r.Findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(r.Findings))
	}
}

func TestWarnsOnMismatchedTokens(t *testing.T) {
	r := run(t, "mismatch")
	if !r.OK {
		t.Fatal("expected ok=true (warning only)")
	}
	f := findByCode(r.Findings, "MISMATCHED_THEME_TOKENS")
	if f == nil {
		t.Fatal("expected MISMATCHED_THEME_TOKENS finding")
	}
	if f.Level != "warning" {
		t.Fatalf("level = %q, want warning", f.Level)
	}
}

func TestFlagsLumoUtilityUnderAura(t *testing.T) {
	r := run(t, "lumo-utility")
	if r.OK {
		t.Fatal("expected ok=false")
	}
	f := findByCode(r.Findings, "LUMO_UTILITY_WITHOUT_LUMO_THEME")
	if f == nil {
		t.Fatal("expected LUMO_UTILITY_WITHOUT_LUMO_THEME finding")
	}
	if f.Level != "error" {
		t.Fatalf("level = %q, want error", f.Level)
	}
	if f.Confidence != "high" {
		t.Fatalf("confidence = %q, want high", f.Confidence)
	}
}

func TestDoesNotFlagLumoUtilityUnderLumo(t *testing.T) {
	r := run(t, "lumo-utility-ok")
	if f := findByCode(r.Findings, "LUMO_UTILITY_WITHOUT_LUMO_THEME"); f != nil {
		t.Fatal("LumoUtility is valid under the Lumo theme; should not be flagged")
	}
}

func TestNoOpWhenThemeIndeterminate(t *testing.T) {
	r := run(t, "indeterminate")
	if !r.OK {
		t.Fatal("expected ok=true (correctness out of scope here)")
	}
	if len(r.ThemesLoaded) != 0 {
		t.Fatalf("themesLoaded = %v, want empty", r.ThemesLoaded)
	}
	f := findByCode(r.Findings, "THEME_INDETERMINATE")
	if f == nil {
		t.Fatal("expected THEME_INDETERMINATE info finding")
	}
	if f.Level != "info" {
		t.Fatalf("level = %q, want info", f.Level)
	}
	for _, x := range r.Findings {
		if x.Level == "error" || x.Level == "warning" {
			t.Fatalf("unexpected %s finding when theme is unknown: %s", x.Level, x.Code)
		}
	}
}

func TestUsageErrorForMissingDir(t *testing.T) {
	r := run(t, "does-not-exist")
	if r.UsageError == "" {
		t.Fatal("expected a usage error for a missing directory")
	}
}
