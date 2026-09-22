package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runHook feeds payload as stdin to `hook post-tool-use` and returns stdout.
// A build descriptor is dropped into the fixture first so the project-root
// resolver scopes the scan to that fixture alone (the fixtures otherwise share
// no pom.xml, which would make the resolver walk up to the whole repo).
func runHook(t *testing.T, fixture, payload string) string {
	t.Helper()
	root := filepath.Join(fixturesDir(t), fixture)
	pom := filepath.Join(root, "pom.xml")
	if err := os.WriteFile(pom, []byte("<project/>"), 0o644); err != nil {
		t.Fatalf("write pom: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(pom) })

	// Interpolate the fixture's absolute path into the payload's $FILE placeholder.
	// A .css payload points at the fixture's stylesheet, a .java one at its
	// Application class, so the hook's file-type gate sees the right extension.
	edited := filepath.Join(root, "src", "main", "java", "com", "example", "Application.java")
	if strings.Contains(payload, "--aura-") || strings.Contains(payload, "--lumo-") {
		edited = filepath.Join(root, "frontend", "themes", "my-theme", "styles.css")
	}
	in := strings.NewReader(strings.ReplaceAll(payload, "$FILE", edited))

	var out strings.Builder
	if code := RunHook([]string{"post-tool-use"}, in, &out, root); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	return out.String()
}

func TestHookFlagsStylingEditInMixedProject(t *testing.T) {
	out := runHook(t, "mixed", `{"tool_input":{"file_path":"$FILE","new_string":"btn.getStyle().set(\"color\",\"red\");"}}`)
	if !strings.Contains(out, "MULTIPLE_BASE_THEMES") {
		t.Fatalf("expected mixing finding in output, got: %q", out)
	}
	if !strings.Contains(out, `"hookEventName": "PostToolUse"`) {
		t.Fatalf("expected PostToolUse hook envelope, got: %q", out)
	}
}

func TestHookSilentOnNonStylingJavaEdit(t *testing.T) {
	out := runHook(t, "mixed", `{"tool_input":{"file_path":"$FILE","new_string":"int total = a + b;"}}`)
	if out != "" {
		t.Fatalf("expected silence for non-styling edit, got: %q", out)
	}
}

func TestHookSilentOnCleanProject(t *testing.T) {
	out := runHook(t, "clean", `{"tool_input":{"file_path":"$FILE","new_string":"addClassName(\"foo\");"}}`)
	if out != "" {
		t.Fatalf("expected silence for clean project, got: %q", out)
	}
}

func TestHookDetectsStylingInMultiEdit(t *testing.T) {
	out := runHook(t, "mixed", `{"tool_input":{"file_path":"$FILE","edits":[{"new_string":"nope"},{"new_string":"addClassName(\"x\")"}]}}`)
	if !strings.Contains(out, "MULTIPLE_BASE_THEMES") {
		t.Fatalf("expected MultiEdit styling to trigger check, got: %q", out)
	}
}

func TestHookIgnoresNonStyleableFiles(t *testing.T) {
	var out strings.Builder
	in := strings.NewReader(`{"tool_input":{"file_path":"/x/readme.txt","content":"getStyle()"}}`)
	if code := RunHook([]string{"post-tool-use"}, in, &out, "/x"); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if out.String() != "" {
		t.Fatalf("expected silence for .txt, got: %q", out.String())
	}
}

func TestHookSilentOnMalformedStdin(t *testing.T) {
	var out strings.Builder
	if code := RunHook([]string{"post-tool-use"}, strings.NewReader("not json"), &out, "."); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if out.String() != "" {
		t.Fatalf("expected silence for malformed stdin, got: %q", out.String())
	}
}

func TestHookUnknownSubcommandIsSilent(t *testing.T) {
	var out strings.Builder
	if code := RunHook([]string{"bogus"}, strings.NewReader("{}"), &out, "."); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if out.String() != "" {
		t.Fatalf("expected silence for unknown subcommand, got: %q", out.String())
	}
}

func TestHookFlagsAuraMisuseAfterCSSEdit(t *testing.T) {
	// Any .css edit counts as a styling change, so gate 2 does not apply here.
	out := runHook(t, "aura-readonly", `{"tool_input":{"file_path":"$FILE","content":"--aura-background-color: #fff;"}}`)
	if !strings.Contains(out, "AURA_READONLY_PROPERTY_ASSIGNED") {
		t.Fatalf("expected the Aura usage check to run from the hook, got: %q", out)
	}
	// The envelope is embedded (and therefore JSON-escaped) in additionalContext.
	if !strings.Contains(out, "check-aura-usage") {
		t.Fatalf("expected a check-aura-usage envelope, got: %q", out)
	}
}

func TestHookStaysSilentOnCleanAuraProject(t *testing.T) {
	out := runHook(t, "aura-clean", `{"tool_input":{"file_path":"$FILE","content":"--aura-base-size: 16;"}}`)
	if out != "" {
		t.Fatalf("expected silence for a correctly themed project, got: %q", out)
	}
}

// Warnings are not worth interrupting the agent for; only error-level findings
// make the hook speak.
func TestHookStaysSilentOnAuraWarningsOnly(t *testing.T) {
	out := runHook(t, "aura-surface", `{"tool_input":{"file_path":"$FILE","content":"--aura-surface-level: 3;"}}`)
	if out != "" {
		t.Fatalf("expected silence for warning-only findings, got: %q", out)
	}
}

// The envelope handed to the agent must stay identical to the tool's own --json
// output, including the payload's field order.
func TestHookEnvelopeMatchesToolJSON(t *testing.T) {
	out := runHook(t, "mixed", `{"tool_input":{"file_path":"$FILE","new_string":"addClassName(\"x\")"}}`)
	for _, want := range []string{"check-theme-mixing", "themesLoaded", "tokenPrefixesUsed", "filesScanned", "findings"} {
		if !strings.Contains(out, want) {
			t.Fatalf("envelope is missing %q, got: %q", want, out)
		}
	}
}
