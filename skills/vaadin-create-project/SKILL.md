---
name: vaadin-create-project
description: Bootstrap a new Vaadin project by downloading a fresh skeleton from start.vaadin.com and extracting it into a directory. Use when the user asks to create, scaffold, bootstrap, or start a new Vaadin application or project. Mirrors `npm init vaadin` / create-vaadin.
---

# vaadin-create-project

Create a new Vaadin project from the official start.vaadin.com skeleton.

**This tool reaches the network** (`https://start.vaadin.com`) and **writes files**
into the target directory. Only run it when the user has asked to create a
project, and confirm the target directory with them if it is ambiguous.

## Run it

The launcher is a self-contained native binary bundled in this plugin's `bin/`.
Claude Code puts it on `PATH`, so run it by name (or use its full path,
`"${CLAUDE_PLUGIN_ROOT}/bin/vaadin-agent-tools"`, if it is not on `PATH`):

```bash
vaadin-agent-tools create-project <target-dir> [flags] --json
```

Flags:

- `--name=<id>` — Maven artifactId (default: sanitized basename of `<target-dir>`).
- `--example=flow|none` — include the "Task List" Flow example view, or start
  empty (default: `flow`).
- `--pre` — use the pre-release Vaadin platform version.
- `--overwrite` — if `<target-dir>` exists and is non-empty, replace its contents.

Example — a project with the example view, in `./my-app`:

```bash
vaadin-agent-tools create-project ./my-app --example=flow --json
```

## Read the result

The JSON envelope reports what was created:

```json
{ "tool": "create-project", "ok": true, "project": "my-app", "directory": "…",
  "example": "flow", "preRelease": false, "files": 77, "skeletonUrl": "…" }
```

Exit code: `0` = created · `1` = download or extraction failed (see the `error`
field) · `2` = usage error (missing target directory, or it already exists and
is non-empty without `--overwrite`).

On success, tell the user the project is ready and how to run it:

```bash
cd <target-dir> && mvn
```
