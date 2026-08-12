---
name: vaadin-check-theme-mixing
description: Detect whether a Vaadin project mixes the Aura and Lumo base themes, which produces conflicting styles and unresolved CSS custom properties. Use when reviewing or building a Vaadin UI, switching or configuring a theme, importing theme stylesheets, or when the user reports styles or --lumo-* / --aura-* custom properties not resolving.
---

# vaadin-check-theme-mixing

Run the bundled checker against a Vaadin project and report what it finds.

## Run it

The launcher is a self-contained native binary bundled in this plugin's `bin/` —
no Node.js and no JVM are required, and it does not download or search the
machine for a runtime. Claude Code puts the plugin's `bin/` on `PATH`, so run it
by name (use `.` for `<project-dir>` if the user did not name one):

```bash
vaadin-agent-tools check-theme-mixing <project-dir> --json
```

If it is not on `PATH`, invoke it by its full path instead:
`"${CLAUDE_PLUGIN_ROOT}/bin/vaadin-agent-tools" check-theme-mixing <project-dir> --json`.

## Read the result

The JSON envelope is:

```json
{ "tool": "check-theme-mixing", "ok": true, "themesLoaded": [], "filesScanned": 0, "findings": [] }
```

Exit code: `0` = no error-level findings · `1` = mixing detected · `2` = usage
error (e.g. the directory does not exist).

Report each entry in `findings` with its `code`, `message`, and `evidence`
(each item is `file` + `line` + `snippet`). The codes:

- `MULTIPLE_BASE_THEMES` (error) — both Aura and Lumo are loaded. Keep exactly
  one base theme.
- `LUMO_UTILITY_WITHOUT_LUMO_THEME` (error) — `LumoUtility` is used while Aura is
  the loaded theme; its utility classes are undefined under Aura.
- `MISMATCHED_THEME_TOKENS` (warning) — `--<other>-*` custom properties are used
  under a different base theme and will not resolve.
- `THEME_INDETERMINATE` (info) — no base theme is explicitly loaded, so the
  theme-dependent checks were **skipped**. This is not a pass; say so, and
  suggest confirming which base theme the project loads.
- `LEGACY_THEME_ANNOTATION` (info) — a legacy `@Theme` annotation sits alongside
  `@StyleSheet`-based theming; worth confirming during a Vaadin 24→25 upgrade.

Treat `error` findings as blocking; surface `warning`/`info` findings for the
user to judge rather than failing the task on them.
