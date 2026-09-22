package tools

import (
	"strings"
	"testing"

	"github.com/vaadin/agent-tools/internal/lib"
	"github.com/vaadin/agent-tools/internal/tool"
)

func runAura(t *testing.T, fixture string) auraUsageReport {
	t.Helper()
	return analyzeAuraUsage(tool.Args{Positionals: []string{fixture}, Cwd: fixturesDir(t)})
}

// allByCode returns every finding carrying the given code — the read-only and
// unitless checks emit one finding per offending property, so a code can repeat.
func allByCode(findings []lib.Finding, code string) []lib.Finding {
	var out []lib.Finding
	for _, f := range findings {
		if f.Code == code {
			out = append(out, f)
		}
	}
	return out
}

// propertyReported reports whether a finding message names the given property
// as its subject (messages start with the property name).
func propertyReported(findings []lib.Finding, code, property string) bool {
	for _, f := range allByCode(findings, code) {
		if strings.HasPrefix(f.Message, property+" ") {
			return true
		}
	}
	return false
}

func TestAuraCleanProjectHasNoFindings(t *testing.T) {
	r := runAura(t, "aura-clean")
	if !r.OK {
		t.Fatal("expected ok=true")
	}
	if len(r.Findings) != 0 {
		t.Fatalf("expected no findings, got %d: %+v", len(r.Findings), r.Findings)
	}
	if got := strings.Join(r.ThemesLoaded, ","); got != "aura" {
		t.Fatalf("themesLoaded = %q, want aura", got)
	}
}

func TestAuraFlagsReadOnlyPropertyAssignment(t *testing.T) {
	r := runAura(t, "aura-readonly")
	if r.OK {
		t.Fatal("expected ok=false")
	}
	for _, prop := range []string{
		"--aura-background-color", "--aura-accent-color", "--aura-surface-color",
		"--aura-font-size-m", "--aura-line-height-m", "--aura-red-text", "--aura-neutral",
	} {
		if !propertyReported(r.Findings, "AURA_READONLY_PROPERTY_ASSIGNED", prop) {
			t.Errorf("expected %s to be reported as read-only", prop)
		}
	}

	// The message must name the customizable property to set instead.
	for _, f := range allByCode(r.Findings, "AURA_READONLY_PROPERTY_ASSIGNED") {
		if !strings.Contains(f.Message, "Set --") {
			t.Errorf("finding for %q does not name a replacement property", f.Message)
		}
		if len(f.Evidence) == 0 {
			t.Errorf("finding %q carries no evidence", f.Message)
		}
	}
}

func TestAuraFontFamilyStacksAreWarningsNotErrors(t *testing.T) {
	r := runAura(t, "aura-readonly")
	for _, f := range allByCode(r.Findings, "AURA_READONLY_PROPERTY_ASSIGNED") {
		if !strings.HasPrefix(f.Message, "--aura-font-family-system ") {
			continue
		}
		if f.Level != "warning" {
			t.Fatalf("level = %q for a font stack, want warning", f.Level)
		}
		return
	}
	t.Fatal("expected --aura-font-family-system to be reported")
}

func TestAuraFlagsVaadinPropertiesReassignedByAura(t *testing.T) {
	r := runAura(t, "aura-readonly")
	for _, prop := range []string{"--vaadin-text-color", "--vaadin-border-color"} {
		if !propertyReported(r.Findings, "AURA_READONLY_PROPERTY_ASSIGNED", prop) {
			t.Errorf("expected %s to be reported while Aura is loaded", prop)
		}
	}
}

// The --vaadin-* properties are only read-only because Aura reassigns them, so
// they must stay unreported when the active theme is not Aura.
func TestAuraSkipsVaadinPropertiesWhenAuraIsNotLoaded(t *testing.T) {
	r := runAura(t, "mismatch") // loads Lumo
	if propertyReported(r.Findings, "AURA_READONLY_PROPERTY_ASSIGNED", "--vaadin-text-color") {
		t.Fatal("--vaadin-* properties must only be checked while Aura is the loaded theme")
	}
}

// The nine computed-but-customizable properties are the documented override
// points; flagging them would steer agents away from exactly what they need.
func TestAuraDoesNotFlagCustomizableComputedProperties(t *testing.T) {
	r := runAura(t, "aura-customizable")
	if !r.OK {
		t.Fatalf("expected ok=true, got findings: %+v", r.Findings)
	}
	if len(r.Findings) != 0 {
		t.Fatalf("customizable properties must not be flagged, got: %+v", r.Findings)
	}
}

func TestAuraFlagsSurfacePropertyWithoutSurfaceClass(t *testing.T) {
	r := runAura(t, "aura-surface")
	if !r.OK {
		t.Fatal("expected ok=true (warning only)")
	}
	found := allByCode(r.Findings, "AURA_SURFACE_PROPERTY_WITHOUT_SURFACE_CLASS")
	if len(found) != 1 {
		t.Fatalf("expected one surface finding, got %d", len(found))
	}
	f := found[0]
	if f.Level != "warning" {
		t.Fatalf("level = %q, want warning", f.Level)
	}
	// Only the two offending declarations; the aura-surface-solid selector, the
	// built-in vaadin-card, and the nested rule under .aura-surface are all fine.
	if len(f.Evidence) != 2 {
		t.Fatalf("evidence = %d entries, want 2: %+v", len(f.Evidence), f.Evidence)
	}
	for _, e := range f.Evidence {
		if strings.Contains(e.Snippet, "--aura-surface-level: 2") ||
			strings.Contains(e.Snippet, "--aura-surface-level: 0") ||
			strings.Contains(e.Snippet, "--aura-surface-opacity: 1") {
			t.Fatalf("flagged a valid surface selector: %+v", e)
		}
	}
}

func TestAuraFlagsAccentColorWithoutAccentClass(t *testing.T) {
	r := runAura(t, "aura-accent")
	if !r.OK {
		t.Fatal("expected ok=true (warning only)")
	}
	found := allByCode(r.Findings, "AURA_ACCENT_SURFACE_WITHOUT_ACCENT_CLASS")
	if len(found) != 1 {
		t.Fatalf("expected one accent finding, got %d", len(found))
	}
	if found[0].Level != "warning" {
		t.Fatalf("level = %q, want warning", found[0].Level)
	}
	// html and .aura-accent-green.status-tile are both valid places to set it.
	if len(found[0].Evidence) != 2 {
		t.Fatalf("evidence = %d entries, want 2: %+v", len(found[0].Evidence), found[0].Evidence)
	}
}

func TestAuraFlagsUnitlessLength(t *testing.T) {
	r := runAura(t, "aura-unitless")
	if r.OK {
		t.Fatal("expected ok=false")
	}
	// One per offending property: inset, radius, border-width, and the two
	// item-overlay paddings.
	found := allByCode(r.Findings, "AURA_UNITLESS_LENGTH")
	if len(found) != 5 {
		t.Fatalf("expected 5 unitless findings, got %d: %+v", len(found), found)
	}
	for _, f := range found {
		if f.Level != "error" {
			t.Fatalf("level = %q, want error", f.Level)
		}
	}
	// The number-valued properties in the same rule are unitless by design.
	for _, prop := range []string{
		"--aura-base-size", "--aura-base-radius", "--aura-contrast-level",
		"--aura-surface-level", "--aura-surface-opacity", "--aura-font-weight-regular",
	} {
		if propertyReported(r.Findings, "AURA_UNITLESS_LENGTH", prop) {
			t.Errorf("%s takes a unitless number by design and must not be flagged", prop)
		}
	}
}

func TestAuraFlagsUnknownPropertyReads(t *testing.T) {
	r := runAura(t, "aura-unknown")
	if !r.OK {
		t.Fatal("expected ok=true (warning only)")
	}
	found := allByCode(r.Findings, "AURA_UNKNOWN_PROPERTY")
	if len(found) != 2 {
		t.Fatalf("expected 2 unknown-property findings, got %d: %+v", len(found), found)
	}
	for _, prop := range []string{"--aura-primary-color", "--aura-contrast-5pct"} {
		if !propertyReported(r.Findings, "AURA_UNKNOWN_PROPERTY", prop) {
			t.Errorf("expected %s to be reported as unknown", prop)
		}
	}
	// A property the project defines itself is a known name.
	if propertyReported(r.Findings, "AURA_UNKNOWN_PROPERTY", "--aura-card-padding") {
		t.Error("a project-defined --aura-* property must not be reported as unknown")
	}
}

// An unknown name that is only assigned (never read) is how a project defines
// its own token — the check must fire on reads alone.
func TestAuraDoesNotFlagUnknownPropertyAssignments(t *testing.T) {
	r := runAura(t, "clean") // assigns --aura-primary-color, never reads it
	if found := allByCode(r.Findings, "AURA_UNKNOWN_PROPERTY"); len(found) != 0 {
		t.Fatalf("assignments must not be reported as unknown reads: %+v", found)
	}
}

func TestAuraUsageErrorForMissingDir(t *testing.T) {
	r := runAura(t, "does-not-exist")
	if r.UsageError == "" {
		t.Fatal("expected a usage error for a missing directory")
	}
}

// Every read-only property must name a replacement that is itself customizable,
// or the tool would send an agent from one read-only property to another.
func TestAuraReplacementAdviceNeverNamesAReadOnlyProperty(t *testing.T) {
	check := func(name string, ro auraReadOnly) {
		for _, word := range strings.FieldsFunc(ro.instead, func(r rune) bool {
			return r != '-' && (r < 'a' || r > 'z')
		}) {
			if !strings.HasPrefix(word, "--") {
				continue
			}
			if _, bad := auraReadOnlyProperties[word]; bad {
				t.Errorf("%s: advice names the read-only property %s", name, word)
			}
			if _, bad := auraReassignedVaadinProperties[word]; bad {
				t.Errorf("%s: advice names the read-only property %s", name, word)
			}
			if !auraKnownProperties[word] && !strings.HasPrefix(word, "--vaadin-") {
				t.Errorf("%s: advice names %s, which Aura does not define", name, word)
			}
		}
	}
	for name, ro := range auraReadOnlyProperties {
		check(name, ro)
	}
	for name, ro := range auraReassignedVaadinProperties {
		check(name, ro)
	}
}

// The curated tables must stay inside the measured property set.
func TestAuraCuratedListsAreSubsetsOfTheKnownProperties(t *testing.T) {
	for name := range auraReadOnlyProperties {
		if !auraKnownProperties[name] {
			t.Errorf("%s is listed as read-only but is not a known Aura property", name)
		}
	}
	for name := range auraLengthProperties {
		if !auraKnownProperties[name] {
			t.Errorf("%s is listed as a length property but is not a known Aura property", name)
		}
	}
	for _, name := range append(append([]string{}, auraSurfaceProperties...), auraAccentInputProperties...) {
		if !auraKnownProperties[name] {
			t.Errorf("%s is checked but is not a known Aura property", name)
		}
	}
}

// Commented-out theme loading must not activate the Aura-gated checks.
func TestAuraIgnoresCommentedOutThemeLoading(t *testing.T) {
	r := runAura(t, "aura-commented-theme")
	if len(r.ThemesLoaded) != 0 {
		t.Fatalf("themesLoaded = %v, want empty — the @StyleSheet/@import are commented out", r.ThemesLoaded)
	}
	if propertyReported(r.Findings, "AURA_READONLY_PROPERTY_ASSIGNED", "--vaadin-text-color") {
		t.Fatal("--vaadin-* must not be reported when no base theme is loaded")
	}
	if !r.OK {
		t.Fatalf("expected ok=true, got findings: %+v", r.Findings)
	}
}

// !important is case-insensitive and allows whitespace, and a comment marker
// inside a string literal must not hide the declarations after it.
func TestAuraUnitlessSeesThroughImportantAndStrings(t *testing.T) {
	r := runAura(t, "aura-unitless")
	for _, prop := range []string{
		"--aura-app-layout-border-width",     // 0 !IMPORTANT
		"--aura-item-overlay-padding-inline", // 0 ! important
		"--aura-item-overlay-padding-block",  // after a content: "/*" string
	} {
		if !propertyReported(r.Findings, "AURA_UNITLESS_LENGTH", prop) {
			t.Errorf("expected %s to be reported as unitless", prop)
		}
	}
}
