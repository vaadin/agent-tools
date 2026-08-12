package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/vaadin/agent-tools/internal/lib"
	"github.com/vaadin/agent-tools/internal/tool"
)

// CheckThemeMixing is a 1:1 port of src/tools/check-theme-mixing.js.
var CheckThemeMixing = tool.Descriptor{
	Name:    "check-theme-mixing",
	Summary: "Detect whether a Vaadin project mixes the Aura and Lumo themes.",
	Usage: `vaadin-agent-tools check-theme-mixing [projectDir]

Scans a Vaadin project for signs that both the Aura and Lumo base themes are in
use at the same time, which leads to conflicting styles and unresolved CSS
custom properties.

Arguments:
  projectDir   Path to the Vaadin project root (default: current directory)

Detection signals:
  - @StyleSheet(Aura.STYLESHEET) / @StyleSheet(Lumo.STYLESHEET) in Java sources
  - @import of aura/aura.css or lumo/lumo.css in reusable-theme stylesheets
  - Usage of --aura-* vs --lumo-* CSS custom properties
  - Usage of the LumoUtility class in Java (only works under the Lumo theme)

Exit codes:
  0  no error-level findings
  1  mixing detected (error-level finding)
  2  usage error (e.g. projectDir does not exist)`,
	Run: runCheckThemeMixing,
}

// themeMixingReport is the typed result of the analysis. The json-tagged fields
// become the tool's JSON payload; OK/UsageError are CLI control fields.
type themeMixingReport struct {
	OK                bool          `json:"-"`
	UsageError        string        `json:"-"`
	ThemesLoaded      []string      `json:"themesLoaded"`
	TokenPrefixesUsed []string      `json:"tokenPrefixesUsed"`
	FilesScanned      int           `json:"filesScanned"`
	Findings          []lib.Finding `json:"findings"`
}

func runCheckThemeMixing(args tool.Args) tool.Result {
	r := analyzeThemeMixing(args)
	if r.UsageError != "" {
		return tool.Result{UsageError: r.UsageError}
	}
	return tool.Result{OK: r.OK, Payload: r, Human: renderThemeMixingHuman(r)}
}

func renderThemeMixingHuman(r themeMixingReport) string {
	var out []string
	out = append(out, "# check-theme-mixing")

	loaded := "(none detected)"
	if len(r.ThemesLoaded) > 0 {
		loaded = strings.Join(r.ThemesLoaded, ", ")
	}
	out = append(out, "themes loaded: "+loaded)
	out = append(out, fmt.Sprintf("files scanned: %d", r.FilesScanned))
	out = append(out, "")

	if len(r.Findings) == 0 {
		out = append(out, "✓ No issues found.")
	} else {
		for _, f := range r.Findings {
			marker := "ℹ"
			switch f.Level {
			case "error":
				marker = "✗"
			case "warning":
				marker = "⚠"
			}
			out = append(out, fmt.Sprintf("%s [%s] %s (confidence: %s)", marker, f.Level, f.Code, f.Confidence))
			out = append(out, "  "+f.Message)
			for _, e := range f.Evidence {
				out = append(out, fmt.Sprintf("    %s:%d  %s", e.File, e.Line, e.Snippet))
			}
			out = append(out, "")
		}
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n")
}

// --- detection patterns -----------------------------------------------------
//
// Go's regexp engine (RE2) has no backreferences, so the JS pattern that used
// \1 to require the same theme name twice (aura/aura.css) is split into one
// explicit pattern per theme instead.
var (
	// @StyleSheet(Aura.STYLESHEET), or with either the annotation or the theme
	// class fully qualified, e.g.
	// @com.vaadin.flow.server.StyleSheet(com.vaadin.flow.theme.aura.Aura.STYLESHEET).
	// The optional (?:[\w.]*\.)? qualifiers require a trailing dot, so they match
	// a package prefix without matching a different annotation (e.g. @MyStyleSheet).
	javaStyleSheetRe = regexp.MustCompile(`@(?:[\w.]*\.)?StyleSheet\s*\(\s*(?:[\w.]*\.)?(Aura|Lumo)\.STYLESHEET`)
	// Legacy Flow theming mechanism (Vaadin 24 and earlier), qualified or not.
	javaLegacyThemeRe = regexp.MustCompile(`@(?:[\w.]*\.)?Theme\s*\(`)
	// LumoUtility emits Lumo-specific utility CSS class names (e.g.
	// LumoUtility.Margin.MEDIUM) that the Aura theme does not define.
	javaLumoUtilityRe = regexp.MustCompile(`\bLumoUtility\b`)

	// @import '.../aura/aura.css' or "lumo/lumo.css"
	cssImportAuraRe = regexp.MustCompile(`@import\s+["'][^"']*/aura/aura\.css["']`)
	cssImportLumoRe = regexp.MustCompile(`@import\s+["'][^"']*/lumo/lumo\.css["']`)
	// CSS custom-property prefixes
	auraTokenRe = regexp.MustCompile(`--aura-[\w-]+`)
	lumoTokenRe = regexp.MustCompile(`--lumo-[\w-]+`)
)

func lineOf(content string, index int) int {
	line := 1
	for i := 0; i < index && i < len(content); i++ {
		if content[i] == '\n' {
			line++
		}
	}
	return line
}

func snippetAt(content string, index int) string {
	start := strings.LastIndexByte(content[:index], '\n') + 1
	end := strings.IndexByte(content[index:], '\n')
	if end == -1 {
		end = len(content)
	} else {
		end += index
	}
	return content[start:end]
}

func analyzeThemeMixing(args tool.Args) themeMixingReport {
	arg := "."
	if len(args.Positionals) > 0 {
		arg = args.Positionals[0]
	}
	target := arg
	if !filepath.IsAbs(target) {
		target = filepath.Join(args.Cwd, target)
	}

	if info, err := os.Stat(target); err != nil || !info.IsDir() {
		return themeMixingReport{UsageError: "Project directory not found: " + target}
	}

	files := lib.Walk(target, []string{".java", ".css"})

	// themeName -> evidence
	loaded := map[string][]lib.Evidence{"aura": {}, "lumo": {}}
	tokens := map[string][]lib.Evidence{"aura": {}, "lumo": {}}
	var legacyTheme []lib.Evidence
	var lumoUtility []lib.Evidence

	rel := func(f string) string {
		if r, err := filepath.Rel(target, f); err == nil {
			return r
		}
		return f
	}

	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		content := string(b)
		ext := strings.ToLower(filepath.Ext(file))

		switch ext {
		case ".java":
			for _, m := range javaStyleSheetRe.FindAllStringSubmatchIndex(content, -1) {
				theme := strings.ToLower(content[m[2]:m[3]])
				idx := m[0]
				loaded[theme] = append(loaded[theme],
					lib.NewEvidence(rel(file), lineOf(content, idx), snippetAt(content, idx)))
			}
			for _, m := range javaLegacyThemeRe.FindAllStringIndex(content, -1) {
				idx := m[0]
				legacyTheme = append(legacyTheme,
					lib.NewEvidence(rel(file), lineOf(content, idx), snippetAt(content, idx)))
			}
			// Report at most one LumoUtility hit per file to keep evidence tidy.
			if loc := javaLumoUtilityRe.FindStringIndex(content); loc != nil {
				idx := loc[0]
				lumoUtility = append(lumoUtility,
					lib.NewEvidence(rel(file), lineOf(content, idx), snippetAt(content, idx)))
			}
		case ".css":
			for _, m := range cssImportAuraRe.FindAllStringIndex(content, -1) {
				idx := m[0]
				loaded["aura"] = append(loaded["aura"],
					lib.NewEvidence(rel(file), lineOf(content, idx), snippetAt(content, idx)))
			}
			for _, m := range cssImportLumoRe.FindAllStringIndex(content, -1) {
				idx := m[0]
				loaded["lumo"] = append(loaded["lumo"],
					lib.NewEvidence(rel(file), lineOf(content, idx), snippetAt(content, idx)))
			}
			// Record only the first token match per file per prefix to keep evidence tidy.
			if loc := auraTokenRe.FindStringIndex(content); loc != nil {
				idx := loc[0]
				tokens["aura"] = append(tokens["aura"],
					lib.NewEvidence(rel(file), lineOf(content, idx), snippetAt(content, idx)))
			}
			if loc := lumoTokenRe.FindStringIndex(content); loc != nil {
				idx := loc[0]
				tokens["lumo"] = append(tokens["lumo"],
					lib.NewEvidence(rel(file), lineOf(content, idx), snippetAt(content, idx)))
			}
		}
	}

	// Fixed aura,lumo order mirrors Object.keys(loaded) insertion order in JS.
	themesLoaded := []string{}
	tokenPrefixesUsed := []string{}
	for _, t := range []string{"aura", "lumo"} {
		if len(loaded[t]) > 0 {
			themesLoaded = append(themesLoaded, t)
		}
		if len(tokens[t]) > 0 {
			tokenPrefixesUsed = append(tokenPrefixesUsed, t)
		}
	}

	findings := []lib.Finding{}

	// 1. High-confidence: both base themes explicitly loaded.
	if len(themesLoaded) > 1 {
		ev := append(append([]lib.Evidence{}, loaded["aura"]...), loaded["lumo"]...)
		findings = append(findings, lib.NewFinding(
			"error", "MULTIPLE_BASE_THEMES",
			"Both the Aura and Lumo base themes are loaded. Load exactly one base theme.",
			"high", ev))
	}

	// The checks below depend on knowing which base theme is active. We only know
	// that when exactly one base theme is explicitly loaded. If none is loaded the
	// project may use a custom or base-styles theme, and ensuring correctness is
	// outside this tool's scope — so it is a no-op rather than a guess.
	if len(themesLoaded) == 1 {
		active := themesLoaded[0]
		other := "lumo"
		if active == "lumo" {
			other = "aura"
		}

		// Tokens from a theme other than the active one will not resolve.
		if len(tokens[other]) > 0 {
			findings = append(findings, lib.NewFinding(
				"warning", "MISMATCHED_THEME_TOKENS",
				"The "+active+" theme is loaded, but --"+other+"-* CSS custom properties are used. "+
					"These will not resolve unless the "+other+" theme is also loaded.",
				"medium", tokens[other]))
		}

		// LumoUtility emits Lumo-specific CSS utility classes that Aura lacks.
		if active == "aura" && len(lumoUtility) > 0 {
			findings = append(findings, lib.NewFinding(
				"error", "LUMO_UTILITY_WITHOUT_LUMO_THEME",
				"LumoUtility is used while the Aura theme is loaded. LumoUtility emits "+
					"Lumo-specific CSS utility classes that Aura does not define.",
				"high", lumoUtility))
		}
	} else if len(themesLoaded) == 0 && (len(lumoUtility) > 0 || len(tokenPrefixesUsed) > 0) {
		// Theme-dependent usage exists but no base theme is explicitly loaded, so we
		// cannot confidently say whether Aura, Lumo, or a custom theme is active.
		// Surface this as info (no error) so it is clear the checks were skipped,
		// not that everything passed.
		ev := append(append(append([]lib.Evidence{}, lumoUtility...), tokens["aura"]...), tokens["lumo"]...)
		findings = append(findings, lib.NewFinding(
			"info", "THEME_INDETERMINATE",
			"No base theme is explicitly loaded via @StyleSheet or a theme @import, so "+
				"the active theme could not be determined (it may be a custom theme). "+
				"Theme-dependent checks were skipped.",
			"low", ev))
	}

	// Informational: legacy @Theme alongside new @StyleSheet theming.
	if len(legacyTheme) > 0 && len(themesLoaded) > 0 {
		findings = append(findings, lib.NewFinding(
			"info", "LEGACY_THEME_ANNOTATION",
			"A legacy @Theme annotation is present alongside @StyleSheet-based theming. "+
				"Confirm this is intentional when upgrading from Vaadin 24.",
			"low", legacyTheme))
	}

	ok := true
	for _, f := range findings {
		if f.Level == "error" {
			ok = false
			break
		}
	}

	return themeMixingReport{
		OK:                ok,
		ThemesLoaded:      themesLoaded,
		TokenPrefixesUsed: tokenPrefixesUsed,
		FilesScanned:      len(files),
		Findings:          findings,
	}
}
