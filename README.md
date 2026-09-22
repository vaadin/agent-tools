# vaadin-agent-tools

A collection of Vaadin tools that AI agents (and humans) can run to inspect and
validate Vaadin projects.

It ships two ways, both from this one repository:

1. A **self-contained native CLI** — no Node.js and no JVM required at runtime,
   and it never searches the machine for one.
2. A **Claude Code / Codex plugin** that exposes the tools as agent **skills**.

## Why a native binary

A Vaadin machine is not guaranteed to have Node.js, and reaching into
`~/.vaadin/node` to borrow Vaadin's copy makes a launcher look like it's hunting
for something to execute. The tools are therefore compiled ahead of time, per
platform, into small self-contained binaries (Go, ~2 MB each). The plugin bundles
those binaries and a tiny selector that picks the right one by `uname` — it only
ever runs code inside this repository, and needs no runtime install, no network,
and no `$HOME` probing.

## Layout

This repository **is** the plugin checkout, with Claude Code and Codex manifests,
the Go sources, and the shared test fixtures alongside each other:

```
.
├── .codex-plugin/
│   └── plugin.json         # Codex plugin manifest
├── .claude-plugin/
│   ├── plugin.json          # Claude Code plugin manifest
│   └── marketplace.json     # self-marketplace, for trying it from a checkout
├── bin/
│   ├── vaadin-agent-tools       # POSIX arch selector (run via ${CLAUDE_PLUGIN_ROOT})
│   ├── vaadin-agent-tools.bat   # Windows selector
│   └── platform/                # native binaries (built by go/build.sh, committed)
├── skills/                 # one SKILL.md per tool (the agent surface)
│   ├── vaadin-check-aura-usage/SKILL.md
│   ├── vaadin-check-theme-mixing/SKILL.md
│   └── vaadin-create-project/SKILL.md
├── hooks/                  # Claude Code hooks shipped by the plugin
│   └── hooks.json          # PostToolUse: styling checks after styling edits
├── go/                      # the CLI implementation (source of the binaries)
│   ├── main.go  build.sh  go.mod
│   └── internal/{cli,tool,tools,lib}/…
├── scripts/                 # maintenance scripts
│   └── derive-aura-properties.sh   # refresh the Aura property tables per release
└── test/fixtures/           # sample projects, shared by the Go tests
```

## Install as a plugin

### Claude Code

For real distribution the plugin is published through
[`vaadin/agent-marketplace`](https://github.com/vaadin/agent-marketplace)
alongside `vaadin-skills`. To try it from a local checkout, this repo doubles as
a one-plugin dev marketplace:

```shell
/plugin marketplace add /path/to/agent-tools
/plugin install vaadin-agent-tools@vaadin-agent-tools-dev
```

Then just ask — the skills trigger by description (e.g. "create a new Vaadin
project" or "check this project for theme mixing") and the agent runs the tool.

The prebuilt binaries are committed under `bin/platform/`, so a fresh checkout is
ready to run — no build step needed to try it.

### Codex

For real distribution, publish the plugin through
[`vaadin/agent-marketplace`](https://github.com/vaadin/agent-marketplace)
alongside `vaadin-skills`. After the `vaadin-agent-tools` entry is published
there, install it from that marketplace:

```shell
codex plugin marketplace add vaadin/agent-marketplace --ref main
codex plugin add vaadin-agent-tools@vaadin-marketplace
```

To pick up later changes, refresh Git-backed marketplace snapshots:

```shell
codex plugin marketplace upgrade
```

Codex reads this repository's `.codex-plugin/plugin.json` and loads the shared
`skills/` directory from the installed plugin. The prebuilt binaries are
committed under `bin/platform/`, so a fresh install is ready to run without a
build step.

## Run the CLI directly

The plugin is a thin wrapper around a normal command line, which you can also run
by hand or in CI:

```shell
bin/vaadin-agent-tools <tool> [args] [--json]
bin/vaadin-agent-tools list
```

Every tool supports `--json` for machine-readable output (recommended for agents)
and exits non-zero when it finds a problem, so it slots into CI and agent
workflows.

## Tools

### `create-project`

Bootstraps a new Vaadin project by downloading a skeleton from
[start.vaadin.com](https://start.vaadin.com) and extracting it — the core of
[`create-vaadin`](https://www.npmjs.com/package/create-vaadin) (`npm init
vaadin`), minus the interactive prompts and IDE launch.

```shell
bin/vaadin-agent-tools create-project ./my-app
bin/vaadin-agent-tools create-project ./my-app --example=flow --pre --json
```

Flags:

- `--name=<id>` — Maven artifactId (default: sanitized basename of the target dir)
- `--example=flow|none` — include the "Task List" Flow example view (default: `flow`)
- `--pre` — use the pre-release Vaadin platform version
- `--overwrite` — if the target exists and is non-empty, replace its contents

This tool reaches the network and writes files. Exit codes: `0` created · `1`
download or extraction failed · `2` usage error.

### `check-theme-mixing`

Detects whether a Vaadin project mixes the **Aura** and **Lumo** themes, which
leads to conflicting styles and CSS custom properties that fail to resolve.

```shell
bin/vaadin-agent-tools check-theme-mixing ./my-project
bin/vaadin-agent-tools check-theme-mixing ./my-project --json
```

Detection signals:

- `@StyleSheet(Aura.STYLESHEET)` / `@StyleSheet(Lumo.STYLESHEET)` in Java sources
- `@import` of `aura/aura.css` or `lumo/lumo.css` in reusable-theme stylesheets
- Usage of `--aura-*` vs `--lumo-*` CSS custom properties
- Usage of the `LumoUtility` class in Java, whose utility CSS classes only work
  under the Lumo theme (not Aura)

If no base theme is explicitly loaded, the active theme cannot be determined (it
may be a custom or base-styles theme). In that case the theme-dependent checks
are a **no-op** — the tool reports a `THEME_INDETERMINATE` info finding and exits
`0` rather than guessing. Ensuring correctness there is outside the tool's scope.

Exit codes: `0` no error-level findings · `1` mixing detected · `2` usage error.

### `check-aura-usage`

Validates how a project uses the **Aura** theme's custom properties — in its CSS,
and in the inline styles its Java sources set through `Style`. Doc search can
tell an agent the rule; this tells it that the stylesheet, or the Flow view, it
just generated breaks the rule.

```shell
bin/vaadin-agent-tools check-aura-usage ./my-project
bin/vaadin-agent-tools check-aura-usage ./my-project --json
```

| Code | Level | Scans | What it catches |
| --- | --- | --- | --- |
| `AURA_READONLY_PROPERTY_ASSIGNED` | error | CSS + Java | A property Aura computes is assigned. Most are defined with `light-dark(…)`, so assigning one value discards *both* color-scheme values and dark mode stops working. The message names the customizable property to set instead. The two font stacks (`--aura-font-family-system`, `--aura-font-family-instrument-sans`) are a warning: redefining them is wrong usage rather than a dark-mode bug. |
| `AURA_SURFACE_PROPERTY_WITHOUT_SURFACE_CLASS` | warning | CSS | `--aura-surface-level` / `--aura-surface-opacity` set on a selector that carries neither `aura-surface` nor `aura-surface-solid` and is not one of the built-in components that use the surface color. Silently does nothing. |
| `AURA_ACCENT_SURFACE_WITHOUT_ACCENT_CLASS` | warning | CSS | `--aura-accent-color-light` / `-dark` set on a selector Aura does not recompute the accent-derived colors on, so `--aura-accent-surface` keeps its inherited tint. Silently does nothing. |
| `AURA_UNITLESS_LENGTH` | error | CSS + Java | An Aura length property given a bare number (`--aura-app-layout-inset: 0`). It feeds a `calc()`, so it needs a unit even at zero. |
| `AURA_UNKNOWN_PROPERTY` | warning | CSS | `var()` reads an `--aura-*` property that neither the theme nor the project defines — a typo or a hallucinated token. Fires on *reads* only; a project may define its own `--aura-`-prefixed properties. |

Exit codes: `0` no error-level findings · `1` error-level findings · `2` usage
error.

#### Java inline styles

The same mistakes are as reachable from a Flow view as from a stylesheet, so the
two checks that need no selector context also run over what `.java` sources
assign through `com.vaadin.flow.dom.Style`:

```java
box.getStyle().set("--aura-background-color", "#fff");   // dark mode silently breaks
box.getStyle().set("--aura-app-layout-inset", "0");      // calc() silently breaks
```

`Style.set(String, String)` is the only `Style` member that can assign a custom
property — the rest are typed setters for standard properties — so the matcher
keys on a `.set("--…", …)` / `.bind("--…", …)` call. What that means at the edges:

- **Non-literal value** (`set("--aura-accent-color", accent)`, `bind(…, signal)`,
  `set(…, "0" + unit)`) — the property name is still checked, so
  `AURA_READONLY_PROPERTY_ASSIGNED` still fires; `AURA_UNITLESS_LENGTH` has
  nothing to read and stays quiet.
- **`Style.remove("--aura-…")`** — not an assignment, not reported.
- **A snippet quoted in a text block** (`"""…"""`) — documentation, not code, so
  its contents are blanked before the scan. The name harvesting above still sees
  them: a token named in a CSS string the project injects is one it knows.
- **`setAttribute("style", "--aura-x: 0")`** — a raw style string, deliberately
  not parsed. Mistakes written that way go unreported.
- **A non-`Style` `.set("--…", …)`** — reported, knowingly. Requiring the `--`
  prefix makes it rare, and the line is worth a look either way.

The three remaining codes stay CSS-only, and not by omission:
`AURA_SURFACE_PROPERTY_WITHOUT_SURFACE_CLASS` and
`AURA_ACCENT_SURFACE_WITHOUT_ACCENT_CLASS` need the class names on the *same*
element, which the documented Java idiom puts in a different statement
(`addClassNames(…)` on a local variable, then `getStyle().set(…)`); correlating
those means tracking a variable through a method body, and getting it wrong means
false positives on exactly the correct code. `AURA_UNKNOWN_PROPERTY` fires on
*reads*, and `Style.set` is a write.

Hilla/React views are a third surface with the same failure mode
(`<div style={{ '--aura-surface-level': 2 }} />`); `.tsx` / `.ts` are not walked
yet.

#### The Aura property tables

The property names come from `@vaadin/aura` and are embedded in
[`go/internal/tools/auradata.go`](go/internal/tools/auradata.go) —
77 names measured from `@vaadin/aura@25.3.0-rc1` (74 the theme defines plus 3 it
only reads through a fallback). Re-derive them and diff against the embedded
table with:

```shell
sh scripts/derive-aura-properties.sh [version]
```

The *names* are derivable; the **write-safety classification is not**. Nine
properties are computed from other properties and are still the documented,
intended override points — `--aura-neutral-light` / `-dark`,
`--aura-accent-text-color-light` / `-dark`,
`--aura-accent-contrast-color-light` / `-dark`,
`--aura-overlay-outline-color`, `--aura-overlay-inner-outline-color` and
`--aura-shadow-color`. A naive "is it computed?" scan would flag exactly the
properties an agent needs most, so the split follows the `Read-only` and
`light-dark()` badges in the Vaadin Aura reference pages instead, and is
maintained by hand. `test/fixtures/aura-customizable` is a regression test for
that false-positive trap.

## Hooks

The plugin ships one Claude Code hook, wired up in
[`hooks/hooks.json`](hooks/hooks.json) and referenced from the plugin manifest.

**PostToolUse → styling checks.** After the agent edits a file (`Edit`, `Write`,
or `MultiEdit`), the hook runs the styling checks — `check-theme-mixing` and
`check-aura-usage` — and, only when one of them finds an error-level problem,
feeds the findings back to the agent so it can fix them. The scope is
deliberately narrow so the hook stays quiet — it speaks only when **all three**
gates pass:

1. The edited file is `.css` or `.java` (anything else is ignored).
2. For `.java`, the text the edit *introduced* mentions a Vaadin styling API
   (`getStyle(`, `*ClassName(s)`, `@CssImport` / `@StyleSheet` / `@Theme`,
   `LumoUtility`, `--lumo-` / `--aura-`, `*ThemeName(s)`, `getThemeList(`). Any
   `.css` edit qualifies.
3. A check reports an error-level finding for the edited file's project. Clean
   projects, warnings, and indeterminate results produce no output, and the edit
   is never blocked.

The hook logic lives in the native binary (`vaadin-agent-tools hook
post-tool-use`, reading the PostToolUse event on stdin), so it runs identically
on macOS, Linux, and Windows with no `bash`, `jq`, or PowerShell dependency. The
plugin isn't on `PATH`, so [`hooks/hooks.json`](hooks/hooks.json) invokes the
selector through `${CLAUDE_PLUGIN_ROOT}`, the plugin's install directory:

```json
"command": "\"${CLAUDE_PLUGIN_ROOT}/bin/vaadin-agent-tools\" hook post-tool-use"
```

You can test it by hand:

```shell
echo '{"tool_input":{"file_path":"/abs/path/App.java","new_string":"btn.getStyle().set(\"color\",\"red\");"}}' \
  | bin/vaadin-agent-tools hook post-tool-use
```

## CLI contract

The command-line surface is the stable interface — treat it as the contract, not
the implementation behind it:

- **Invocation:** `vaadin-agent-tools <tool> [args] [--json] [-h|--help] [-v|--version]`
- **JSON output** (`--json`): `{ "tool", "ok", ...result }`, or
  `{ "tool", "ok": false, "usageError" }` for a usage error.
- **Exit codes:** `0` no error-level findings · `1` findings/error · `2` usage
  error.

The implementation is Go today, reached only through this contract (the command
name, args, JSON shape, and exit codes) so a future reimplementation could drop
in behind the same `vaadin-agent-tools` command without changing how agents or CI
call it.

## Building

Cross-compile the binaries for every supported platform into `bin/platform/`:

```shell
sh go/build.sh
```

Targets: `linux/amd64`, `linux/arm64`, `windows/amd64`, and a universal macOS
binary (`darwin`, arm64 + amd64 via `lipo`; falls back to per-arch binaries when
`lipo` is unavailable). Binaries are built with `CGO_ENABLED=0` and stripped
(`-ldflags "-s -w" -trimpath`) — small and self-contained, with no packing.

## Adding a tool

1. Create `go/internal/tools/<name>.go` defining a `tool.Descriptor`
   (`Name`, `Summary`, `Usage`, and a `Run(tool.Args) tool.Result` function).
2. Register it in the `registry` slice in
   [`go/internal/cli/cli.go`](go/internal/cli/cli.go).
3. Add a `skills/vaadin-<name>/SKILL.md` so agents discover and run it by
   description. Prefix the skill `name` with `vaadin-` so it does not collide
   with skills from other plugins (the CLI subcommand stays unprefixed).

Use the shared helpers in [`go/internal/lib`](go/internal/lib) — `Walk` for
scanning project files, `NewFinding` / `NewEvidence` for the standard finding
shape — to keep output consistent across tools.

## Development

```shell
cd go
go test ./...
go vet ./...
```

The tests reuse the sample projects in [`test/fixtures`](test/fixtures).

## Distribution

The native binaries in `bin/platform/` are **committed to the repository** so that
a git-source install (a fresh clone, or the marketplace) has a runnable plugin
with no build step. When you change anything under `go/`, rebuild and commit the
binaries in the same change so they stay in sync with the source:

```shell
sh go/build.sh && git add bin/platform
```

Because the plugin manifest is at the repository root, a marketplace entry can use
a plain `url` source pointing at this repo — the same pattern `vaadin-skills`
uses. (If the committed binaries ever become too heavy for the repo, the
alternative is to keep them out and ship a release archive referenced from the
marketplace entry with `{ "source": "archive", "url": "…", "sha256": "…" }`.)

## License

Apache-2.0
