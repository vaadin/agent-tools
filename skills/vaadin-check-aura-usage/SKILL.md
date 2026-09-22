---
name: vaadin-check-aura-usage
description: Validate how a Vaadin project uses the Aura theme's custom properties — in its CSS, and in the inline styles its Java sources set through getStyle().set — catching assignments the browser silently discards: read-only computed properties, surface and accent properties set on the wrong selector, unitless lengths, and unknown --aura-* names. Use after writing or editing Vaadin CSS or a Flow view that sets an --aura-* property from Java, when customizing the Aura theme, setting colors, fonts, spacing, surface levels or accent colors, or when the user reports that a style change had no effect or that dark mode stopped working.
---

# vaadin-check-aura-usage

Run the bundled checker against a Vaadin project and report what it finds. Doc
search can tell you the Aura rule; this tells you whether the stylesheet — or the
Flow view — you just wrote breaks it.

## What it scans

`.css` files, and the `--aura-*` / `--vaadin-*` properties `.java` sources assign
inline through `com.vaadin.flow.dom.Style`:

```java
box.getStyle().set("--aura-background-color", "#fff");   // reported: dark mode breaks
box.getStyle().set("--aura-app-layout-inset", "0");      // reported: calc() breaks
```

Two codes carry over to Java, three stay CSS-only — see the list below. A value
the call does not pass as a string literal (a variable, a `bind(…)` signal, a
concatenation) still has its property name checked; only the value-dependent
check goes quiet. `Style.remove(…)` is not an assignment and is not reported, and
a whole style string passed to `setAttribute("style", …)` is not parsed at all —
mistakes written that way go unreported. `.tsx` / `.ts` are not scanned, so a
Hilla view's `style={{ '--aura-…': … }}` is not covered either.

## Run it

The launcher is a self-contained native binary bundled in this plugin's `bin/` —
no Node.js and no JVM are required, and it does not download or search the
machine for a runtime. Claude Code puts the plugin's `bin/` on `PATH`, so run it
by name (use `.` for `<project-dir>` if the user did not name one):

```bash
vaadin-agent-tools check-aura-usage <project-dir> --json
```

If it is not on `PATH`, invoke it by its full path instead:
`"${CLAUDE_PLUGIN_ROOT}/bin/vaadin-agent-tools" check-aura-usage <project-dir> --json`.

## Read the result

The JSON envelope is:

```json
{ "tool": "check-aura-usage", "ok": true, "themesLoaded": ["aura"], "filesScanned": 0, "findings": [] }
```

Exit code: `0` = no error-level findings · `1` = error-level findings · `2` =
usage error (e.g. the directory does not exist).

Report each entry in `findings` with its `code`, `message`, and `evidence`
(each item is `file` + `line` + `snippet`). The codes:

- `AURA_READONLY_PROPERTY_ASSIGNED` (error, `warning` for the two font stacks;
  CSS and Java) — a property Aura computes is being assigned. Most of these are
  defined with `light-dark(...)`, so assigning one value throws away *both*
  color-scheme values and dark mode stops working. The message names the
  customizable property to set instead — use that one.
- `AURA_SURFACE_PROPERTY_WITHOUT_SURFACE_CLASS` (warning; CSS only) —
  `--aura-surface-level` / `--aura-surface-opacity` set on a selector that does
  not use the Aura surface color, where it does nothing. Add the `aura-surface`
  or `aura-surface-solid` class name to the element.
- `AURA_ACCENT_SURFACE_WITHOUT_ACCENT_CLASS` (warning; CSS only) — the accent
  color is changed on a selector Aura does not recompute the accent-derived
  colors on, so `--aura-accent-surface` and friends keep their inherited
  values. Add one of the `aura-accent-*` class names.
- `AURA_UNITLESS_LENGTH` (error; CSS and Java) — an Aura length property was
  given a bare number. It feeds a `calc()`, so it needs a unit even at zero:
  write `0px`.
- `AURA_UNKNOWN_PROPERTY` (warning; CSS only) — `var()` reads an `--aura-*`
  property that neither the theme nor the project defines, so it resolves to
  nothing. Usually a typo or a Lumo-shaped name; check it against the Aura style
  property reference. A name the project sets from Java counts as defined.

The two selector checks are CSS-only because they need the class names on the
element, and Java sets those in a separate statement from the property:

```java
box.addClassNames("aura-surface", "recessed-box");
box.getStyle().set("--aura-surface-level", "-1");   // correct, and not checked
```

So when a Java view sets `--aura-surface-level` / `--aura-surface-opacity` or an
accent color, check yourself that the element carries the matching class name —
the tool does not.

Treat `error` findings as blocking and fix them. Surface `warning` findings for
the user to judge rather than failing the task on them — the two selector checks
are heuristics and a deliberately unusual selector can be a false positive.

`themesLoaded` tells you which base theme the project loads. The seven
`--vaadin-*` properties Aura reassigns are only checked when Aura is the one
loaded theme; if `themesLoaded` is empty or lists `lumo`, say so rather than
implying those were verified.

Pair this with `vaadin-check-theme-mixing` when the project's base theme itself
looks uncertain.
