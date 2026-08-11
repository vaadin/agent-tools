# @vaadin/agent-tools

A collection of Vaadin tools that AI agents (and humans) can run directly with
`npx` to inspect and validate Vaadin projects — no installation required.

## Usage

```sh
npx @vaadin/agent-tools <tool> [args] [--json]
```

List the available tools:

```sh
npx @vaadin/agent-tools list
```

Every tool supports `--json` for machine-readable output (recommended for
agents) and exits non-zero when it finds a problem, so it slots into CI and
agent workflows.

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
