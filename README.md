# @vaadin/agent-tools

A collection of Vaadin tools that AI agents (and humans) can run to inspect and
validate Vaadin projects.

## Usage

If you have a global Node.js install, run it with `npx`:

```sh
npx @vaadin/agent-tools <tool> [args] [--json]
npx @vaadin/agent-tools list
```

But a Vaadin machine is **not guaranteed to have `node`/`npm` on `PATH`** — the
only runtime you can count on is the JVM, and Node, if present at all, is the
copy Vaadin's frontend toolchain downloads under `~/.vaadin/node`. For that case
the package ships launchers in [`bin/`](bin) that locate a runtime themselves:

```sh
# from a checkout or a vendored copy — no global node required
bin/vaadin-agent-tools <tool> [args] [--json]     # macOS / Linux
bin\vaadin-agent-tools.bat <tool> [args] [--json] # Windows
```

The launcher resolves `node` in this order, then execs the bundled CLI:

1. `$VAADIN_AGENT_TOOLS_NODE` — explicit override
2. `node` on `PATH`
3. Node downloaded by Vaadin — `~/.vaadin/node`
4. Project-local `node/`

If none is found it exits `3` with instructions (install Node, or run a Vaadin
frontend build once so Vaadin downloads it).

Every tool supports `--json` for machine-readable output (recommended for
agents) and exits non-zero when it finds a problem, so it slots into CI and
agent workflows.

### Making it available to agents

Because the launcher needs no global `node` and no network at invocation time,
the tools work **vendored on disk** — checked out or bundled into an agent /
plugin at a known path and run via `bin/vaadin-agent-tools`. This is the
recommended distribution for agents (including those in offline sandboxes),
independent of whether the package has been published to npmjs.com. Publishing
and `npx` remain a convenience for humans on machines that already have Node.

## Tools

### `check-theme-mixing`

Detects whether a Vaadin project mixes the **Aura** and **Lumo** themes, which
leads to conflicting styles and CSS custom properties that fail to resolve.

```sh
npx @vaadin/agent-tools check-theme-mixing ./my-project
npx @vaadin/agent-tools check-theme-mixing ./my-project --json
```

Detection signals:

- `@StyleSheet(Aura.STYLESHEET)` / `@StyleSheet(Lumo.STYLESHEET)` in Java sources
- `@import` of `aura/aura.css` or `lumo/lumo.css` in reusable-theme stylesheets
- Usage of `--aura-*` vs `--lumo-*` CSS custom properties
- Usage of the `LumoUtility` class in Java, whose utility CSS classes only
  work under the Lumo theme (not Aura)

If no base theme is explicitly loaded, the active theme cannot be determined
(it may be a custom or base-styles theme). In that case the theme-dependent
checks are a **no-op** — the tool reports a `THEME_INDETERMINATE` info finding
and exits `0` rather than guessing. Ensuring correctness there is outside the
tool's scope.

Exit codes: `0` no error-level findings · `1` mixing detected · `2` usage error.

## CLI contract

The command-line surface is the stable interface — treat it as the contract, not
the JavaScript implementation behind it:

- **Invocation:** `<launcher> <tool> [args] [--json] [-h|--help] [-v|--version]`
- **JSON output** (`--json`): `{ "tool", "ok", ...result }`, or
  `{ "tool", "ok": false, "usageError" }` for a usage error.
- **Exit codes:** `0` no error-level findings · `1` findings/error · `2` usage
  error · `3` no Node.js runtime could be located (launcher only).

The implementation is JavaScript today. It is intentionally reachable only
through this contract (the launcher name, args, JSON shape, and exit codes) so a
future **JVM reimplementation** — the always-present runtime on a Vaadin machine
— can drop in behind the same `vaadin-agent-tools` command without changing how
agents or CI call it.

## Adding a tool

1. Create `src/tools/<name>.js` exporting `name`, `summary`, `usage`, and a
   `run({ positionals, flags, cwd })` function that returns a result object
   (`{ ok, findings, ... }`) or `{ usageError }`.
2. Register the module in the `TOOLS` array in [`src/cli.js`](src/cli.js).

Use the shared helpers in [`src/lib`](src/lib) (`walk` for scanning project
files, `finding`/`evidence` for the standard finding shape) to keep output
consistent across tools.

## Development

```sh
npm test
```

## Relationship to `create-vaadin`

This package is separate from [`create-vaadin`](https://www.npmjs.com/package/create-vaadin)
(the package behind `npm init vaadin`). `create-vaadin` scaffolds new
applications; `@vaadin/agent-tools` inspects and validates existing ones.

## License

Apache-2.0
