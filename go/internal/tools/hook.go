package tools

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

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
// It reads a Claude Code or Codex PostToolUse payload from stdin and, when the
// edit that just happened actually touched Vaadin styling, runs the styling
// checks (check-theme-mixing and check-aura-usage) against each edited file's own
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
// tool_input's shape varies by tool. Claude Code's tools carry an absolute
// `file_path`: Write has `content`, Edit has `new_string`, MultiEdit has
// `edits[].new_string`. Codex edits through apply_patch, whose only input is
// `command`, the raw patch text, with paths relative to `cwd`.
type postToolUsePayload struct {
	Cwd       string `json:"cwd"`
	ToolInput struct {
		FilePath  string `json:"file_path"`
		NewString string `json:"new_string"`
		Content   string `json:"content"`
		Edits     []struct {
			NewString string `json:"new_string"`
		} `json:"edits"`
		Command string `json:"command"`
	} `json:"tool_input"`
}

// fileEdit is one file an edit touched, with the text the edit introduced there.
type fileEdit struct {
	path       string
	introduced string
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

	// Codex patch paths are relative to the session's directory, which the
	// payload names; it is also where Codex starts the hook, so this only matters
	// if the two ever differ.
	if filepath.IsAbs(p.Cwd) {
		cwd = p.Cwd
	}

	// --- Gate 1: file type, and for .java, gate 2: the edit touched styling. ---
	// Each check is scoped to the edited file's own project. One Codex patch can
	// touch files in several projects, so collect each distinct project once.
	var projects []string
	seen := map[string]bool{}
	for _, e := range editsFrom(p) {
		if !touchesStyling(e) {
			continue
		}
		if project := projectRootFor(e.path, cwd); !seen[project] {
			seen[project] = true
			projects = append(projects, project)
		}
	}

	// --- Gate 3: only speak when a check finds an error-level problem. ---
	var sections []string
	for _, project := range projects {
		found := checkProject(project, cwd)
		if len(found) == 0 {
			continue
		}
		if len(projects) > 1 {
			sections = append(sections, "Project: "+project)
		}
		sections = append(sections, found...)
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

// editsFrom returns the files the tool call edited. A Claude Code edit names one
// file; a Codex apply_patch can add or update several.
func editsFrom(p postToolUsePayload) []fileEdit {
	in := p.ToolInput
	if in.FilePath != "" {
		var changed strings.Builder
		changed.WriteString(in.NewString)
		changed.WriteByte('\n')
		changed.WriteString(in.Content)
		for _, e := range in.Edits {
			changed.WriteByte('\n')
			changed.WriteString(e.NewString)
		}
		return []fileEdit{{path: in.FilePath, introduced: changed.String()}}
	}
	if in.Command != "" {
		return parseApplyPatch(in.Command)
	}
	return nil
}

// touchesStyling applies gates 1 and 2 to one edited file.
func touchesStyling(e fileEdit) bool {
	switch strings.ToLower(filepath.Ext(e.path)) {
	case ".css":
		// Any CSS edit is a styling change.
		return true
	case ".java":
		return javaStylingRe.MatchString(e.introduced)
	default:
		return false
	}
}

// checkProject runs the styling checks against one project and returns the
// envelope of each check that found an error-level problem.
func checkProject(project, cwd string) []string {
	args := tool.Args{Positionals: []string{project}, JSON: true, Cwd: cwd}
	var sections []string
	if r := analyzeThemeMixing(args); r.UsageError == "" && !r.OK {
		sections = append(sections, envelopeFor("check-theme-mixing", r.OK, r))
	}
	if r := analyzeAuraUsage(args); r.UsageError == "" && !r.OK {
		sections = append(sections, envelopeFor("check-aura-usage", r.OK, r))
	}
	return sections
}

// The apply_patch envelope markers, as codex-rs/apply-patch/src/parser.rs spells
// them.
const (
	patchBeginMarker  = "*** Begin Patch"
	patchEndMarker    = "*** End Patch"
	patchAddMarker    = "*** Add File: "
	patchDeleteMarker = "*** Delete File: "
	patchUpdateMarker = "*** Update File: "
	patchMoveToMarker = "*** Move to: "
)

// parseApplyPatch returns the files a Codex apply_patch envelope adds or updates,
// each with the text its `+` lines introduce. It reads the envelope the way
// Codex's own parser does, including its leniency about whitespace around the
// markers and a heredoc wrapper around the whole patch. Unlike that parser it
// never fails: Codex has already applied the patch by the time this hook runs,
// so a line it does not recognize is skipped. A deleted file introduces nothing
// and is left out. A moved file is reported at its new path.
func parseApplyPatch(patch string) []fileEdit {
	type patchedFile struct {
		path  string
		added []string
	}
	var files []patchedFile
	const (
		outside = iota // before the first hunk, or in a deleted file's hunk
		adding
		updating
	)
	mode, started, expectMove := outside, false, false

lines:
	for _, line := range strings.Split(patch, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if !started {
			started = strings.TrimSpace(line) == patchBeginMarker
			continue
		}
		// Codex trims a header line on both sides, except inside an update hunk,
		// where leading whitespace marks a context line.
		header := strings.TrimSpace(line)
		if mode == updating {
			header = strings.TrimRightFunc(line, unicode.IsSpace)
		}
		atMove := expectMove
		expectMove = false

		switch {
		case header == patchEndMarker:
			break lines
		case strings.HasPrefix(header, patchAddMarker):
			files = append(files, patchedFile{path: strings.TrimPrefix(header, patchAddMarker)})
			mode = adding
		case strings.HasPrefix(header, patchDeleteMarker):
			mode = outside
		case strings.HasPrefix(header, patchUpdateMarker):
			files = append(files, patchedFile{path: strings.TrimPrefix(header, patchUpdateMarker)})
			mode, expectMove = updating, true
		case atMove && strings.HasPrefix(header, patchMoveToMarker):
			files[len(files)-1].path = strings.TrimPrefix(header, patchMoveToMarker)
		case mode != outside && strings.HasPrefix(line, "+"):
			last := &files[len(files)-1]
			last.added = append(last.added, line[1:])
		}
	}

	edits := make([]fileEdit, len(files))
	for i, f := range files {
		edits[i] = fileEdit{path: f.path, introduced: strings.Join(f.added, "\n")}
	}
	return edits
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
