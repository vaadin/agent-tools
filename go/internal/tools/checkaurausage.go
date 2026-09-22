package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/vaadin/agent-tools/internal/lib"
	"github.com/vaadin/agent-tools/internal/tool"
)

// CheckAuraUsage validates how a project uses the Aura theme's CSS custom
// properties, in its stylesheets and in the inline styles its Java sources set.
// Doc search can tell an agent the rule; this tells it that the stylesheet — or
// the Flow view — it just wrote breaks the rule.
var CheckAuraUsage = tool.Descriptor{
	Name:    "check-aura-usage",
	Summary: "Validate Aura theme usage in a Vaadin project's stylesheets and Java inline styles.",
	Usage: `vaadin-agent-tools check-aura-usage [projectDir]

Scans a Vaadin project's CSS, and the inline styles its Java sources set through
Style.set / Style.bind, for misuse of the Aura theme's custom properties —
assignments that are silently discarded, or that break the light/dark color
schemes.

Arguments:
  projectDir   Path to the Vaadin project root (default: current directory)

Checks:
  AURA_READONLY_PROPERTY_ASSIGNED             (error)   a computed Aura property is assigned
  AURA_SURFACE_PROPERTY_WITHOUT_SURFACE_CLASS (warning) surface property on a non-surface selector
  AURA_ACCENT_SURFACE_WITHOUT_ACCENT_CLASS    (warning) accent color on a non-accent selector
  AURA_UNITLESS_LENGTH                        (error)   a length property given a unitless number
  AURA_UNKNOWN_PROPERTY                       (warning) var() reads an --aura-* property nothing defines

The two selector checks and AURA_UNKNOWN_PROPERTY are CSS-only: the first two
need the class names on the element, which a Java Style.set call does not carry,
and the last one fires on reads, which Style.set is not.

Exit codes:
  0  no error-level findings
  1  error-level findings
  2  usage error (e.g. projectDir does not exist)`,
	Run: runCheckAuraUsage,
}

// auraUsageReport is the typed result of the analysis. The json-tagged fields
// become the tool's JSON payload; OK/UsageError are CLI control fields.
type auraUsageReport struct {
	OK           bool          `json:"-"`
	UsageError   string        `json:"-"`
	ThemesLoaded []string      `json:"themesLoaded"`
	FilesScanned int           `json:"filesScanned"`
	Findings     []lib.Finding `json:"findings"`
}

func runCheckAuraUsage(args tool.Args) tool.Result {
	r := analyzeAuraUsage(args)
	if r.UsageError != "" {
		return tool.Result{UsageError: r.UsageError}
	}
	return tool.Result{OK: r.OK, Payload: r, Human: renderAuraUsageHuman(r)}
}

func renderAuraUsageHuman(r auraUsageReport) string {
	loaded := "(none detected)"
	if len(r.ThemesLoaded) > 0 {
		loaded = strings.Join(r.ThemesLoaded, ", ")
	}
	out := []string{
		"# check-aura-usage",
		"themes loaded: " + loaded,
		fmt.Sprintf("files scanned: %d", r.FilesScanned),
		"",
	}
	out = append(out, lib.RenderFindings(r.Findings)...)
	return strings.TrimRight(strings.Join(out, "\n"), "\n")
}

// The Java sources are read by two passes that look at the same string literals
// for opposite purposes, so keep them apart when changing either:
//
//   - javaCustomPropertyRe harvests NAMES the project itself uses, to suppress
//     AURA_UNKNOWN_PROPERTY for a project's own tokens. It is deliberately broad:
//     any --aura-* literal anywhere counts as "the project knows this name".
//   - javaStyleSetRe recognizes ASSIGNMENTS, which are then checked exactly like
//     a CSS declaration. It is deliberately narrow: only a Style.set/bind call.
//
// The two never contradict each other: a read-only property is in
// auraKnownProperties regardless, so harvesting its name suppresses nothing.

// javaCustomPropertyRe finds an --aura-* custom property named in a Java string
// literal, e.g. getStyle().set("--aura-card-padding", "1rem"). Such a property is
// defined by the project even though no stylesheet declares it, so it must not be
// reported as unknown.
var javaCustomPropertyRe = regexp.MustCompile(`"(--aura-[\w-]+)"`)

// javaStyleSetRe finds a custom property assigned from Java through
// com.vaadin.flow.dom.Style — getStyle().set("--aura-app-layout-inset", "0") or
// getElement().getStyle().set(…). Style.set(String, String) is the only Style
// member that can assign a custom property; the rest are typed setters for
// standard properties. Style.bind(String, Signal<String>) is matched too: its
// value is not readable here, but the property name is.
//
// Whitespace is allowed after the dot — Java permits it, and it is also what a
// blanked-out comment between the dot and the method name leaves behind.
//
// The value group is optional, and matches only a lone literal with no escape
// sequences in it that closes the call. Group 2 therefore either is the whole
// value verbatim, or did not participate at all — which is how a non-literal
// value (a variable, a signal, a "0" + unit concatenation) is told apart from a
// literal one, so the value-dependent check can stay quiet rather than guess.
//
// Deliberately NOT matched:
//
//   - Style.remove("--aura-…"), which is not an assignment.
//   - anything inside a text block, which is a snippet being quoted rather than
//     code the browser will run (see the blanking pass at the call site).
//   - setAttribute("style", "--aura-x: 0"), a raw style string: the first
//     argument is "style", so recognizing it would mean CSS-parsing the second
//     one for a spelling Flow code rarely uses. Its mistakes go unreported.
//
// Matched by accident: any non-Style API of the same shape, e.g.
// config.set("--aura-…", "…"). Requiring the -- prefix on the first argument
// keeps that rare, and such a line is worth a look either way.
var javaStyleSetRe = regexp.MustCompile(`\.\s*(?:set|bind)\s*\(\s*"(--[\w-]+)"\s*,\s*(?:"([^"\\]*)"\s*\))?`)

// unitlessNumberRe matches a bare number with no CSS unit.
var unitlessNumberRe = regexp.MustCompile(`^[+-]?(?:\d+(?:\.\d+)?|\.\d+)$`)

// importantRe matches a trailing !important. CSS lets it be written in any case
// and with whitespace after the bang, so "0 ! IMPORTANT" must strip down to "0".
var importantRe = regexp.MustCompile(`(?i)\s*!\s*important\s*$`)

// auraDeclaration is one custom-property declaration together with where it came
// from, so a finding can point at it. A declaration is either a CSS declaration
// or a Java Style.set/bind call; the two fields below say which, because not
// every check applies to both.
type auraDeclaration struct {
	lib.CSSDeclaration
	file    string // path relative to the project root
	line    int
	snippet string
	// fromJava marks a Style.set/bind call. Selectors is empty for one — the
	// element's class names live in other statements — so the two checks that
	// need selector context skip these.
	fromJava bool
	// valueKnown is false when the assigned value could not be read: a Java call
	// passing a variable or a Signal rather than a string literal. The property
	// name is still checkable; the value is not.
	valueKnown bool
}

func (d auraDeclaration) evidence() lib.Evidence {
	return lib.NewEvidence(d.file, d.line, d.snippet)
}

func analyzeAuraUsage(args tool.Args) auraUsageReport {
	arg := "."
	if len(args.Positionals) > 0 {
		arg = args.Positionals[0]
	}
	target := arg
	if !filepath.IsAbs(target) {
		target = filepath.Join(args.Cwd, target)
	}

	if info, err := os.Stat(target); err != nil || !info.IsDir() {
		return auraUsageReport{UsageError: "Project directory not found: " + target}
	}

	files := lib.Walk(target, []string{".java", ".css"})

	var decls []auraDeclaration
	var reads []auraDeclaration // Property/Offset carry the var() name and position
	loaded := map[string]bool{}
	// Custom properties the project itself defines, which are therefore known
	// names even when Aura does not define them.
	projectDefined := map[string]bool{}

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

		switch strings.ToLower(filepath.Ext(file)) {
		case ".java":
			// Scan with comments blanked, so a commented-out @StyleSheet does not
			// look like a loaded theme. Blanking preserves byte offsets.
			code := lib.BlankJavaComments(content)
			for _, m := range javaStyleSheetRe.FindAllStringSubmatch(code, -1) {
				loaded[strings.ToLower(m[1])] = true
			}
			for _, m := range javaCustomPropertyRe.FindAllStringSubmatch(code, -1) {
				projectDefined[m[1]] = true
			}
			// Assignments are read from a second view with text blocks blanked
			// too: a snippet quoted in a """…""" string is documentation, and
			// reporting it would flag a warning about wrong code as wrong code.
			// The harvesting above deliberately still sees those names — a token
			// named in a CSS string the project injects is one the project knows.
			stmts := lib.BlankJavaTextBlocks(code)
			for _, m := range javaStyleSetRe.FindAllStringSubmatchIndex(stmts, -1) {
				name := stmts[m[2]:m[3]]
				value := ""
				valueKnown := m[4] >= 0 // the value group did not participate otherwise
				if valueKnown {
					value = stmts[m[4]:m[5]]
				}
				decls = append(decls, auraDeclaration{
					CSSDeclaration: lib.CSSDeclaration{Property: name, Value: value, Offset: m[2]},
					file:           rel(file),
					line:           lineOf(content, m[2]),
					snippet:        snippetAt(content, m[2]),
					fromJava:       true,
					valueKnown:     valueKnown,
				})
			}

		case ".css":
			css := lib.BlankComments(content)
			if cssImportAuraRe.MatchString(css) {
				loaded["aura"] = true
			}
			if cssImportLumoRe.MatchString(css) {
				loaded["lumo"] = true
			}

			cssDecls, cssReads := lib.ParseCSS(css)
			for _, d := range cssDecls {
				projectDefined[d.Property] = true
				decls = append(decls, auraDeclaration{
					CSSDeclaration: d,
					file:           rel(file),
					line:           lineOf(content, d.Offset),
					snippet:        snippetAt(content, d.Offset),
					valueKnown:     true,
				})
			}
			for _, v := range cssReads {
				reads = append(reads, auraDeclaration{
					CSSDeclaration: lib.CSSDeclaration{Property: v.Name, Offset: v.Offset},
					file:           rel(file),
					line:           lineOf(content, v.Offset),
					snippet:        snippetAt(content, v.Offset),
				})
			}
		}
	}

	themesLoaded := []string{}
	for _, t := range []string{"aura", "lumo"} {
		if loaded[t] {
			themesLoaded = append(themesLoaded, t)
		}
	}
	auraIsActive := len(themesLoaded) == 1 && themesLoaded[0] == "aura"

	var findings []lib.Finding
	findings = append(findings, auraReadOnlyFindings(decls, auraIsActive)...)
	findings = append(findings, auraSurfaceFindings(decls)...)
	findings = append(findings, auraAccentFindings(decls)...)
	findings = append(findings, auraUnitlessFindings(decls)...)
	findings = append(findings, auraUnknownFindings(reads, projectDefined)...)

	if findings == nil {
		findings = []lib.Finding{}
	}

	ok := true
	for _, f := range findings {
		if f.Level == "error" {
			ok = false
			break
		}
	}

	return auraUsageReport{
		OK:           ok,
		ThemesLoaded: themesLoaded,
		FilesScanned: len(files),
		Findings:     findings,
	}
}

// --- check 1: a computed (read-only) property is assigned --------------------

func auraReadOnlyFindings(decls []auraDeclaration, auraIsActive bool) []lib.Finding {
	// One finding per offending property, so each message can name the
	// customizable property to set in its place.
	byProperty := map[string][]lib.Evidence{}
	spec := map[string]auraReadOnly{}
	lead := map[string]string{}

	for _, d := range decls {
		ro, known := auraReadOnlyProperties[d.Property]
		what := "is a read-only Aura property"
		if !known {
			// The --vaadin-* base-style properties are only read-only because Aura
			// reassigns them, so they are only checked while Aura is the loaded theme.
			if !auraIsActive {
				continue
			}
			ro, known = auraReassignedVaadinProperties[d.Property]
			if !known {
				continue
			}
			what = "is a base-style property that Aura reassigns with a computed value, " +
				"so it is read-only while Aura is the loaded theme"
		}
		spec[d.Property] = ro
		lead[d.Property] = what
		byProperty[d.Property] = append(byProperty[d.Property], d.evidence())
	}

	var out []lib.Finding
	for _, prop := range sortedKeys(byProperty) {
		ro := spec[prop]
		out = append(out, lib.NewFinding(ro.level, "AURA_READONLY_PROPERTY_ASSIGNED",
			fmt.Sprintf("%s %s. %s Set %s instead.", prop, lead[prop], ro.reason, ro.instead),
			"high", byProperty[prop]))
	}
	return out
}

// --- check 2: surface property on a selector that is not a surface -----------

func auraSurfaceFindings(decls []auraDeclaration) []lib.Finding {
	var evidence []lib.Evidence
	for _, d := range decls {
		if d.fromJava {
			// Both selector checks ask which class names the element carries, and
			// Java puts that in a different statement from the assignment:
			//
			//	box.addClassNames("aura-surface", "recessed-box");
			//	box.getStyle().set("--aura-surface-level", "-1");
			//
			// Answering it means following a variable through a method body. Until
			// something does that, staying quiet beats warning about correct code.
			continue
		}
		if !contains(auraSurfaceProperties, d.Property) {
			continue
		}
		if lib.SelectorChainMatches(d.Selectors, auraSurfaceRespondingSelectors) {
			continue
		}
		evidence = append(evidence, d.evidence())
	}
	if len(evidence) == 0 {
		return nil
	}
	return []lib.Finding{lib.NewFinding("warning", "AURA_SURFACE_PROPERTY_WITHOUT_SURFACE_CLASS",
		"--aura-surface-level / --aura-surface-opacity only take effect on an element that uses the "+
			"Aura surface color. Add the aura-surface (or aura-surface-solid) class name to the element, "+
			"or set the property on one of the built-in components that use the surface color — otherwise "+
			"the declaration does nothing.",
		"medium", evidence)}
}

// --- check 3: accent color on a selector that does not recompute it ----------

func auraAccentFindings(decls []auraDeclaration) []lib.Finding {
	var evidence []lib.Evidence
	for _, d := range decls {
		if d.fromJava {
			continue // no selector to judge, as in check 2
		}
		if !contains(auraAccentInputProperties, d.Property) {
			continue
		}
		if lib.SelectorChainMatches(d.Selectors, auraAccentSelectors) {
			continue
		}
		evidence = append(evidence, d.evidence())
	}
	if len(evidence) == 0 {
		return nil
	}
	return []lib.Finding{lib.NewFinding("warning", "AURA_ACCENT_SURFACE_WITHOUT_ACCENT_CLASS",
		"Aura only recomputes the accent-derived colors (--aura-accent-surface, --aura-accent-text-color, "+
			"--aura-accent-contrast-color, …) on elements that carry an accent class name. Add one of "+
			auraAccentClassNames+" to the element — otherwise changing --aura-accent-color-light / "+
			"--aura-accent-color-dark here does nothing.",
		"medium", evidence)}
}

// --- check 4: a length property given a unitless number ----------------------

func auraUnitlessFindings(decls []auraDeclaration) []lib.Finding {
	byProperty := map[string][]lib.Evidence{}
	for _, d := range decls {
		if !auraLengthProperties[d.Property] {
			continue
		}
		if !d.valueKnown {
			continue // a Java call passing a variable or a signal: nothing to inspect
		}
		value := strings.TrimSpace(importantRe.ReplaceAllString(d.Value, ""))
		if !unitlessNumberRe.MatchString(value) {
			continue
		}
		byProperty[d.Property] = append(byProperty[d.Property], d.evidence())
	}

	var out []lib.Finding
	for _, prop := range sortedKeys(byProperty) {
		out = append(out, lib.NewFinding("error", "AURA_UNITLESS_LENGTH",
			fmt.Sprintf("%s is a CSS length that Aura feeds into calc()/clamp(), so it needs a unit even "+
				"when the value is zero — write 0px, not 0.", prop),
			"high", byProperty[prop]))
	}
	return out
}

// --- check 5: var() reads an --aura-* property nothing defines ---------------

func auraUnknownFindings(reads []auraDeclaration, projectDefined map[string]bool) []lib.Finding {
	byProperty := map[string][]lib.Evidence{}
	for _, r := range reads {
		name := r.Property
		if !strings.HasPrefix(name, "--aura-") {
			continue
		}
		if auraKnownProperties[name] || projectDefined[name] {
			continue
		}
		byProperty[name] = append(byProperty[name], r.evidence())
	}

	var out []lib.Finding
	for _, prop := range sortedKeys(byProperty) {
		out = append(out, lib.NewFinding("warning", "AURA_UNKNOWN_PROPERTY",
			fmt.Sprintf("%s is read here but the Aura theme does not define it and nothing in this project "+
				"does either, so it resolves to nothing. Check the spelling against the Aura style property "+
				"reference, or define the property.", prop),
			"high", byProperty[prop]))
	}
	return out
}

// --- small helpers -----------------------------------------------------------

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string][]lib.Evidence) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
