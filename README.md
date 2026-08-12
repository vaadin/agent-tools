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

This repository **is** the plugin (manifest at the root), with the Go sources and
the shared test fixtures alongside it:

```
.
├── .claude-plugin/
│   ├── plugin.json          # plugin manifest
│   └── marketplace.json     # self-marketplace, for trying it from a checkout
├── bin/
│   ├── vaadin-agent-tools       # POSIX arch selector (on PATH in Claude Code)
│   ├── vaadin-agent-tools.bat   # Windows selector
│   └── platform/                # native binaries, produced by go/build.sh
├── skills/                 # one SKILL.md per tool (the agent surface)
│   ├── vaadin-check-theme-mixing/SKILL.md
│   └── vaadin-create-project/SKILL.md
├── go/                      # the CLI implementation (source of the binaries)
│   ├── main.go  build.sh  go.mod
│   └── internal/{cli,tool,tools,lib}/…
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

The binaries in `bin/platform/` must exist first — run `sh go/build.sh` once (see
[Building](#building)).

### Codex

Codex has no plugin package format, but it loads the same `SKILL.md`. Copy the
skill into a project's (or your personal) skills directory:

```shell
mkdir -p .codex/skills
cp -R skills/vaadin-check-theme-mixing .codex/skills/
```

The one difference from Claude Code: `${CLAUDE_PLUGIN_ROOT}` is Claude-only. For
Codex, reference the launcher relative to the skill directory instead.

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

The native binaries in `bin/platform/` are `.gitignore`d as build artifacts. To
publish through [`vaadin/agent-marketplace`](https://github.com/vaadin/agent-marketplace),
either:

- **commit `bin/platform/`** (drop it from `.gitignore`) so a git-source install
  fetches the binaries, or
- attach a release archive and reference it from the marketplace entry with
  `{ "source": "archive", "url": "…", "sha256": "…" }`.

Because the plugin manifest is at the repository root, a marketplace entry can use
a plain `url` source pointing at this repo — the same pattern
`vaadin-skills` uses.

## Relationship to `create-vaadin`

This project is separate from
[`create-vaadin`](https://www.npmjs.com/package/create-vaadin) (the package behind
`npm init vaadin`). `create-vaadin` scaffolds new applications;
`vaadin-agent-tools` inspects and validates existing ones.

## License

Apache-2.0
