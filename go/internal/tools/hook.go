package tools

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/vaadin/agent-tools/internal/lib"
	"github.com/vaadin/agent-tools/internal/tool"
)

// RunHook implements the `hook` command surface. It is deliberately kept out of
// the tool registry: hooks read a JSON event on stdin and emit a hook-specific
// JSON contract on stdout, which does not fit the { tool, ok, ... } envelope the
// registry tools produce.
//
// Currently one hook is implemented:
//
//	vaadin-agent-tools hook post-tool-use
//
// It reads a Claude Code PostToolUse payload from stdin and, when the edit that
// just happened actually touched Vaadin styling, runs the styling checks
// (check-theme-mixing and check-aura-usage) against the edited file's own
// project. Only when one of them reports an error-level finding does it print a
// PostToolUse response asking the agent to fix it. In every other case it stays
// completely silent. It always exits 0 so a styling edit is never blocked by
// this hook.
//
// Being part of the native binary, it runs identically on macOS, Linux and
// Windows — no bash, jq or PowerShell required.
func RunHook(args []string, stdin io.Reader, stdout io.Writer, cwd string) int {
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "post-tool-use":
		return runPostToolUseHook(stdin, stdout, cwd)
	default:
		// Misconfiguration, not a normal edit — stay quiet rather than spamming
		// the transcript on every subsequent tool call.
		return 0
	}
}

// postToolUsePayload is the subset of the PostToolUse stdin JSON this hook reads.
// tool_input's shape varies by tool: Write has `content`, Edit has `new_string`,
// MultiEdit has `edits[].new_string`.
type postToolUsePayload struct {
	ToolInput struct {
		FilePath  string `json:"file_path"`
		NewString string `json:"new_string"`
		Content   string `json:"content"`
		Edits     []struct {
			NewString string `json:"new_string"`
		} `json:"edits"`
	} `json:"tool_input"`
}

// postToolUseResponse is the PostToolUse hook output contract. additionalContext
// is surfaced to the agent; we leave the edit itself untouched (non-blocking).
type postToolUseResponse struct {
	HookSpecificOutput struct {
		HookEventName     string `json:"hookEventName"`
		AdditionalContext string `json:"additionalContext"`
	} `json:"hookSpecificOutput"`
}

// javaStylingRe matches the text an edit introduced when that edit touches
// Vaadin styling. Kept narrow on purpose so ordinary business-logic .java edits
// never trigger the check. RE2-compatible (no backreferences).
var javaStylingRe = regexp.MustCompile(
	`getStyle\(` +
		`|(?:add|remove|set|get)ClassNames?[(<]` +
		`|@(?:[\w.]*\.)?CssImport` +
		`|@(?:[\w.]*\.)?StyleSheet` +
		`|@(?:[\w.]*\.)?Theme\b` +
		`|\bLumoUtility\b` +
		`|--lumo-` +
		`|--aura-` +
		`|(?:add|set)ThemeNames?\(` +
		`|getThemeList\(`,
)

func runPostToolUseHook(stdin io.Reader, stdout io.Writer, cwd string) int {
	raw, err := io.ReadAll(stdin)
	if err != nil {
		return 0
	}
	var p postToolUsePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return 0
	}

	file := p.ToolInput.FilePath
	if file == "" {
		return 0
	}

	// --- Gate 1: file type, and for .java, gate 2: the edit touched styling. ---
	switch strings.ToLower(filepath.Ext(file)) {
	case ".css":
		// Any CSS edit is a styling change.
	case ".java":
		var changed strings.Builder
		changed.WriteString(p.ToolInput.NewString)
		changed.WriteByte('\n')
		changed.WriteString(p.ToolInput.Content)
		for _, e := range p.ToolInput.Edits {
			changed.WriteByte('\n')
			changed.WriteString(e.NewString)
		}
		if !javaStylingRe.MatchString(changed.String()) {
			return 0
		}
	default:
		return 0
	}

	// Scope the check to the edited file's own project.
	project := projectRootFor(file, cwd)

	// --- Gate 3: only speak when a check finds an error-level problem. ---
	args := tool.Args{Positionals: []string{project}, JSON: true, Cwd: cwd}
	var sections []string

	if r := analyzeThemeMixing(args); r.UsageError == "" && !r.OK {
		sections = append(sections, envelopeFor("check-theme-mixing", r.OK, r))
	}
	if r := analyzeAuraUsage(args); r.UsageError == "" && !r.OK {
		sections = append(sections, envelopeFor("check-aura-usage", r.OK, r))
	}
	if len(sections) == 0 {
		return 0
	}

	var resp postToolUseResponse
	resp.HookSpecificOutput.HookEventName = "PostToolUse"
	resp.HookSpecificOutput.AdditionalContext =
		"Vaadin styling checks flagged issues after this change. Review the findings " +
			"below and fix the error-level ones:\n" +
			strings.Join(sections, "\n")

	out, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return 0
	}
	_, _ = stdout.Write(out)
	_, _ = io.WriteString(stdout, "\n")
	return 0
}

// projectRootFor returns the nearest ancestor of file that carries a build
// descriptor (pom.xml / build.gradle[.kts]). It falls back to CLAUDE_PROJECT_DIR,
// then to cwd, so a project without a recognized descriptor still gets scanned.
func projectRootFor(file, cwd string) string {
	dir := filepath.Dir(file)
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(cwd, dir)
	}
	for {
		for _, marker := range []string{"pom.xml", "build.gradle", "build.gradle.kts"} {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if env := os.Getenv("CLAUDE_PROJECT_DIR"); env != "" {
		return env
	}
	return cwd
}

// envelopeFor renders the same { "tool", "ok", ...payload } JSON the CLI emits,
// so the context handed to the agent matches that tool's `--json` output.
func envelopeFor(name string, ok bool, payload any) string {
	head := lib.MarshalIndentNoEscape(struct {
		Tool string `json:"tool"`
		OK   bool   `json:"ok"`
	}{name, ok})
	body := lib.MarshalIndentNoEscape(payload)
	if body == "{}" || body == "null" || body == "" {
		return head
	}
	// Splice the payload's own fields in after "ok", keeping their order.
	return strings.TrimSuffix(head, "\n}") + "," + body[1:]
}
