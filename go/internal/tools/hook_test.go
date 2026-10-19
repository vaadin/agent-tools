package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
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
	// A payload that mentions a custom property is a stylesheet edit and points at
	// the fixture's stylesheet; anything else points at its Application class, so
	// the hook's file-type gate sees the right extension. A fixture with no
	// stylesheet at all is a Java-only one, where even a custom property was
	// edited into a .java file.
	edited := filepath.Join(root, "src", "main", "java", "com", "example", "Application.java")
	if strings.Contains(payload, "--aura-") || strings.Contains(payload, "--lumo-") {
		if css := filepath.Join(root, "frontend", "themes", "my-theme", "styles.css"); fileExists(css) {
			edited = css
		}
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

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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

// An inline style set from a Flow view is the same mistake as the stylesheet
// version, and it reaches the hook through the existing .java gate — the edit
// mentions getStyle(, so gate 2 passes without any hook change.
func TestHookFlagsAuraMisuseAfterJavaInlineStyleEdit(t *testing.T) {
	out := runHook(t, "aura-java-inline",
		`{"tool_input":{"file_path":"$FILE","new_string":"box.getStyle().set(\"--aura-background-color\", \"#fff\");"}}`)
	if !strings.Contains(out, "AURA_READONLY_PROPERTY_ASSIGNED") {
		t.Fatalf("expected the Aura usage check to speak after a Java inline-style edit, got: %q", out)
	}
	if !strings.Contains(out, "DashboardView.java") {
		t.Fatalf("expected the finding to point at the Java source, got: %q", out)
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

// runPatchHook feeds a Codex apply_patch PostToolUse payload to the hook. The
// patch's relative paths resolve against cwd, the fixtures directory, which the
// payload names; the hook process itself starts somewhere unrelated, so the test
// also proves the payload's cwd is the one used. Each listed fixture gets a build
// descriptor, as in runHook.
func runPatchHook(t *testing.T, patch string, fixtures ...string) string {
	t.Helper()
	for _, fixture := range fixtures {
		pom := filepath.Join(fixturesDir(t), fixture, "pom.xml")
		if err := os.WriteFile(pom, []byte("<project/>"), 0o644); err != nil {
			t.Fatalf("write pom: %v", err)
		}
		t.Cleanup(func() { _ = os.Remove(pom) })
	}
	payload, err := json.Marshal(map[string]any{
		"hook_event_name": "PostToolUse",
		"cwd":             fixturesDir(t),
		"tool_name":       "apply_patch",
		"tool_input":      map[string]string{"command": patch},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	var out strings.Builder
	if code := RunHook([]string{"post-tool-use"}, strings.NewReader(string(payload)), &out, t.TempDir()); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	return out.String()
}

func TestHookFlagsAuraMisuseAfterCodexCSSPatch(t *testing.T) {
	out := runPatchHook(t, `*** Begin Patch
*** Update File: aura-readonly/frontend/themes/my-theme/styles.css
@@ html {
+  --aura-background-color: #fff;
*** End Patch`, "aura-readonly")
	if !strings.Contains(out, "AURA_READONLY_PROPERTY_ASSIGNED") {
		t.Fatalf("expected the Aura usage check to run after a Codex CSS patch, got: %q", out)
	}
	if strings.Contains(out, "Project: ") {
		t.Fatalf("a single-project patch should not label its sections, got: %q", out)
	}
}

func TestHookFlagsStylingAfterCodexJavaPatch(t *testing.T) {
	out := runPatchHook(t, `*** Begin Patch
*** Update File: mixed/src/main/java/com/example/Application.java
@@ public class Application {
+    void style(Button btn) { btn.getStyle().set("color", "red"); }
*** End Patch`, "mixed")
	if !strings.Contains(out, "MULTIPLE_BASE_THEMES") {
		t.Fatalf("expected mixing finding after a Codex styling patch, got: %q", out)
	}
}

// Only the lines a patch adds count as what it introduced. Styling code that is
// merely context, or that the patch removes, must not wake the hook.
func TestHookSilentOnCodexNonStylingJavaPatch(t *testing.T) {
	out := runPatchHook(t, `*** Begin Patch
*** Update File: mixed/src/main/java/com/example/Application.java
@@ public class Application {
     btn.getStyle().set("color", "red");
-    btn.addClassName("primary");
+    int total = a + b;
*** End Patch`, "mixed")
	if out != "" {
		t.Fatalf("expected silence for a non-styling Codex patch, got: %q", out)
	}
}

// One patch can edit files in different projects: each project is checked once
// and its findings are labelled with the project they belong to. The second path
// is absolute, which apply_patch also accepts.
func TestHookChecksEveryProjectACodexPatchTouches(t *testing.T) {
	css := filepath.Join(fixturesDir(t), "aura-readonly", "frontend", "themes", "my-theme", "styles.css")
	out := runPatchHook(t, `*** Begin Patch
*** Update File: mixed/src/main/java/com/example/Application.java
@@
+    btn.addClassName("primary");
*** Update File: mixed/src/main/java/com/example/Application.java
@@
+    int total = a + b;
*** Update File: `+css+`
@@
+  --aura-background-color: #fff;
*** End Patch`, "mixed", "aura-readonly")
	for _, want := range []string{
		"MULTIPLE_BASE_THEMES",
		"AURA_READONLY_PROPERTY_ASSIGNED",
		"Project: " + filepath.Join(fixturesDir(t), "mixed"),
		"Project: " + filepath.Join(fixturesDir(t), "aura-readonly"),
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in the output, got: %q", want, out)
		}
	}
	if n := strings.Count(out, "Project: "+filepath.Join(fixturesDir(t), "mixed")); n != 1 {
		t.Fatalf("expected the mixed project to be checked once, got %d sections: %q", n, out)
	}
}

// Deleting a stylesheet introduces no styling, even in a project that has
// error-level findings.
func TestHookSilentOnCodexDeleteOnlyPatch(t *testing.T) {
	out := runPatchHook(t, `*** Begin Patch
*** Delete File: aura-readonly/frontend/themes/my-theme/styles.css
*** End Patch`, "aura-readonly")
	if out != "" {
		t.Fatalf("expected silence for a delete-only Codex patch, got: %q", out)
	}
}

func TestParseApplyPatch(t *testing.T) {
	tests := []struct {
		name  string
		patch string
		want  []fileEdit
	}{
		{
			name:  "added file",
			patch: "*** Begin Patch\n*** Add File: a/styles.css\n+html {\n+  color: red;\n+}\n*** End Patch",
			want:  []fileEdit{{path: "a/styles.css", introduced: "html {\n  color: red;\n}"}},
		},
		{
			name:  "update keeps only added lines",
			patch: "*** Begin Patch\n*** Update File: A.java\n@@ class A {\n ctx();\n-old();\n+fresh();\n*** End of File\n*** End Patch",
			want:  []fileEdit{{path: "A.java", introduced: "fresh();"}},
		},
		{
			name:  "move reports the new path",
			patch: "*** Begin Patch\n*** Update File: old.txt\n*** Move to: new.css\n@@\n+x\n*** End Patch",
			want:  []fileEdit{{path: "new.css", introduced: "x"}},
		},
		{
			name:  "late move marker is ignored",
			patch: "*** Begin Patch\n*** Update File: a.txt\n@@\n+x\n*** Move to: b.css\n*** End Patch",
			want:  []fileEdit{{path: "a.txt", introduced: "x"}},
		},
		{
			name:  "deleted file is left out",
			patch: "*** Begin Patch\n*** Delete File: gone.css\n*** Add File: kept.css\n+a\n*** End Patch",
			want:  []fileEdit{{path: "kept.css", introduced: "a"}},
		},
		{
			name:  "heredoc wrapper, padded markers and CRLF",
			patch: "<<'EOF'\r\n  *** Begin Patch \r\n *** Add File: a.css\r\n+b\r\n*** End Patch\r\nEOF\r\n",
			want:  []fileEdit{{path: "a.css", introduced: "b"}},
		},
		{
			name:  "nothing after the end marker",
			patch: "*** Begin Patch\n*** Add File: a.css\n+b\n*** End Patch\n*** Add File: c.css\n+d",
			want:  []fileEdit{{path: "a.css", introduced: "b"}},
		},
		{
			name:  "not a patch",
			patch: "mvn package",
			want:  []fileEdit{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseApplyPatch(tt.patch); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseApplyPatch() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
