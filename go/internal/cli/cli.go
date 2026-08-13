// Package cli implements the command-line surface — the stable contract shared
// with the JavaScript implementation: tool names, argument parsing, the --json
// envelope, and exit codes (0 ok · 1 findings · 2 usage error).
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/vaadin/agent-tools/internal/tool"
	"github.com/vaadin/agent-tools/internal/tools"
)

// jsonIndent marshals v with two-space indentation and without Go's default
// HTML escaping of <, > and & — so source-code snippets in evidence stay
// readable and the output matches JSON.stringify from the JS implementation.
func jsonIndent(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
	return strings.TrimRight(buf.String(), "\n")
}

// Version is the CLI version. Kept in lockstep with the npm package version.
const Version = "0.2.1"

// registry lists the available tools. To add a tool: implement a
// tool.Descriptor in internal/tools and append it here.
var registry = []tool.Descriptor{
	tools.CheckThemeMixing,
	tools.CreateProject,
}

func byName(name string) *tool.Descriptor {
	for i := range registry {
		if registry[i].Name == name {
			return &registry[i]
		}
	}
	return nil
}

type flags struct {
	json    bool
	help    bool
	version bool
}

func parseArgv(argv []string) (flags, []string) {
	var f flags
	var pos []string
	for _, a := range argv {
		switch a {
		case "--json":
			f.json = true
		case "--help", "-h":
			f.help = true
		case "--version", "-v":
			f.version = true
		default:
			pos = append(pos, a)
		}
	}
	return f, pos
}

func topLevelHelp() string {
	lines := []string{
		"vaadin-agent-tools — Vaadin tools for AI agents and humans",
		"",
		"Usage:",
		"  vaadin-agent-tools <tool> [args] [--json]",
		"",
		"Tools:",
	}
	for _, t := range registry {
		lines = append(lines, fmt.Sprintf("  %-22s %s", t.Name, t.Summary))
	}
	lines = append(lines,
		"",
		"Hooks (read a Claude Code event on stdin):",
		"  hook post-tool-use     Flag Aura/Lumo theme mixing after a styling edit",
		"",
		"Global flags:",
		"  --json        Emit machine-readable JSON (recommended for agents)",
		"  -h, --help    Show help (use with a tool for tool-specific help)",
		"  -v, --version Print version",
	)
	return strings.Join(lines, "\n")
}

// envelopeJSON renders { "tool": name, "ok": ok, ...payload }, keeping the
// payload struct's own field order (mirrors the JS `{ tool, ok, ...result }`).
func envelopeJSON(name string, ok bool, payload any) string {
	head := fmt.Sprintf("{\n  %q: %q,\n  %q: %t", "tool", name, "ok", ok)
	body := jsonIndent(payload)
	if body == "{}" || body == "null" || body == "" {
		return head + "\n}"
	}
	// body starts with "{" — replace it (and keep the "\n  ...") after "ok".
	return head + "," + body[1:]
}

// Run executes the CLI with the given args (os.Args[1:]) and returns the process
// exit code.
func Run(argv []string) int {
	f, pos := parseArgv(argv)

	if f.version {
		fmt.Println(Version)
		return 0
	}

	var command string
	if len(pos) > 0 {
		command = pos[0]
		pos = pos[1:]
	}

	// The `hook` command reads a Claude Code hook event on stdin and emits a
	// hook-specific JSON contract on stdout — it does not use the { tool, ok, ... }
	// envelope, so it is handled before the tool registry.
	if command == "hook" {
		cwd, _ := os.Getwd()
		return tools.RunHook(pos, os.Stdin, os.Stdout, cwd)
	}

	if command == "" || command == "help" || command == "list" {
		if f.json {
			type toolInfo struct {
				Name    string `json:"name"`
				Summary string `json:"summary"`
			}
			payload := struct {
				Version string     `json:"version"`
				Tools   []toolInfo `json:"tools"`
			}{Version: Version}
			for _, t := range registry {
				payload.Tools = append(payload.Tools, toolInfo{t.Name, t.Summary})
			}
			fmt.Println(jsonIndent(payload))
		} else {
			fmt.Println(topLevelHelp())
		}
		return 0
	}

	t := byName(command)
	if t == nil {
		fmt.Fprintf(os.Stderr, "Unknown tool: %s\n\n%s\n", command, topLevelHelp())
		return 2
	}

	if f.help {
		usage := t.Usage
		if usage == "" {
			usage = t.Summary
		}
		fmt.Println(usage)
		return 0
	}

	cwd, _ := os.Getwd()
	res := t.Run(tool.Args{Positionals: pos, JSON: f.json, Cwd: cwd})

	if res.UsageError != "" {
		if f.json {
			fmt.Println(jsonIndent(struct {
				Tool       string `json:"tool"`
				OK         bool   `json:"ok"`
				UsageError string `json:"usageError"`
			}{t.Name, false, res.UsageError}))
		} else {
			fmt.Fprintf(os.Stderr, "Error: %s\n", res.UsageError)
		}
		return 2
	}

	if f.json {
		fmt.Println(envelopeJSON(t.Name, res.OK, res.Payload))
	} else {
		fmt.Println(res.Human)
	}

	if res.OK {
		return 0
	}
	return 1
}
