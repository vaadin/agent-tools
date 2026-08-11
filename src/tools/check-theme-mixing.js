import fs from "node:fs";
import path from "node:path";
import { walk } from "../lib/walk.js";
import { finding, evidence } from "../lib/findings.js";

export const name = "check-theme-mixing";
export const summary =
  "Detect whether a Vaadin project mixes the Aura and Lumo themes.";
export const usage = `vaadin-agent-tools check-theme-mixing [projectDir]

Scans a Vaadin project for signs that both the Aura and Lumo base themes are in
use at the same time, which leads to conflicting styles and unresolved CSS
custom properties.

Arguments:
  projectDir   Path to the Vaadin project root (default: current directory)

Detection signals:
  - @StyleSheet(Aura.STYLESHEET) / @StyleSheet(Lumo.STYLESHEET) in Java sources
  - @import of aura/aura.css or lumo/lumo.css in reusable-theme stylesheets
  - Usage of --aura-* vs --lumo-* CSS custom properties
  - Usage of the LumoUtility class in Java (only works under the Lumo theme)

Exit codes:
  0  no error-level findings
  1  mixing detected (error-level finding)
  2  usage error (e.g. projectDir does not exist)`;

// --- detection patterns -----------------------------------------------------

// @StyleSheet(Aura.STYLESHEET) or @StyleSheet(com.vaadin...Aura.STYLESHEET)
const JAVA_STYLESHEET_RE =
  /@StyleSheet\s*\(\s*(?:[\w.]*\.)?(Aura|Lumo)\.STYLESHEET/g;
// Legacy Flow theming mechanism (Vaadin 24 and earlier)
const JAVA_LEGACY_THEME_RE = /@Theme\s*\(/g;
// LumoUtility emits Lumo-specific utility CSS class names (e.g.
// LumoUtility.Margin.MEDIUM) that the Aura theme does not define.
const JAVA_LUMO_UTILITY_RE = /\bLumoUtility\b/g;

// @import '.../aura/aura.css' or "lumo/lumo.css"
const CSS_IMPORT_RE =
  /@import\s+["'][^"']*\/(aura|lumo)\/\1\.css["']/g;
// CSS custom-property prefixes
const AURA_TOKEN_RE = /--aura-[\w-]+/g;
const LUMO_TOKEN_RE = /--lumo-[\w-]+/g;

function lineOf(content, index) {
  let line = 1;
  for (let i = 0; i < index && i < content.length; i++) {
    if (content[i] === "\n") line++;
  }
  return line;
}

function snippetAt(content, index) {
  const start = content.lastIndexOf("\n", index) + 1;
  let end = content.indexOf("\n", index);
  if (end === -1) end = content.length;
  return content.slice(start, end);
}

export function run({ positionals, cwd }) {
  const target = path.resolve(cwd, positionals[0] ?? ".");

  if (!fs.existsSync(target) || !fs.statSync(target).isDirectory()) {
    return {
      usageError: `Project directory not found: ${target}`,
    };
  }

  const files = walk(target, { extensions: [".java", ".css"] });

  // themeName -> [evidence]
  const loaded = { aura: [], lumo: [] };
  const tokens = { aura: [], lumo: [] };
  const legacyTheme = [];
  const lumoUtility = [];

  const rel = (f) => path.relative(target, f);

  for (const file of files) {
    let content;
    try {
      content = fs.readFileSync(file, "utf8");
    } catch {
      continue;
    }
    const ext = path.extname(file).toLowerCase();

    if (ext === ".java") {
      for (const m of content.matchAll(JAVA_STYLESHEET_RE)) {
        const theme = m[1].toLowerCase();
        loaded[theme].push(
          evidence(rel(file), lineOf(content, m.index), snippetAt(content, m.index))
        );
      }
      for (const m of content.matchAll(JAVA_LEGACY_THEME_RE)) {
        legacyTheme.push(
          evidence(rel(file), lineOf(content, m.index), snippetAt(content, m.index))
        );
      }
      // Report at most one LumoUtility hit per file to keep evidence tidy.
      const utility = JAVA_LUMO_UTILITY_RE.exec(content);
      JAVA_LUMO_UTILITY_RE.lastIndex = 0;
      if (utility) {
        lumoUtility.push(
          evidence(rel(file), lineOf(content, utility.index), snippetAt(content, utility.index))
        );
      }
    } else if (ext === ".css") {
      for (const m of content.matchAll(CSS_IMPORT_RE)) {
        const theme = m[1].toLowerCase();
        loaded[theme].push(
          evidence(rel(file), lineOf(content, m.index), snippetAt(content, m.index))
        );
      }
      // Record only the first token match per file per prefix to keep evidence tidy.
      const aura = content.match(AURA_TOKEN_RE);
      if (aura) {
        const idx = content.indexOf(aura[0]);
        tokens.aura.push(
          evidence(rel(file), lineOf(content, idx), snippetAt(content, idx))
        );
      }
      const lumo = content.match(LUMO_TOKEN_RE);
      if (lumo) {
        const idx = content.indexOf(lumo[0]);
        tokens.lumo.push(
          evidence(rel(file), lineOf(content, idx), snippetAt(content, idx))
        );
      }
    }
  }

  const themesLoaded = Object.keys(loaded).filter((t) => loaded[t].length > 0);
  const tokenPrefixesUsed = Object.keys(tokens).filter((t) => tokens[t].length > 0);
  const findings = [];

  // 1. High-confidence: both base themes explicitly loaded.
  if (themesLoaded.length > 1) {
    findings.push(
      finding(
        "error",
        "MULTIPLE_BASE_THEMES",
        "Both the Aura and Lumo base themes are loaded. Load exactly one base theme.",
        {
          confidence: "high",
          evidence: [...loaded.aura, ...loaded.lumo],
        }
      )
    );
  }

  // The checks below depend on knowing which base theme is active. We only know
  // that when exactly one base theme is explicitly loaded. If none is loaded the
  // project may use a custom or base-styles theme, and ensuring correctness is
  // outside this tool's scope — so it is a no-op rather than a guess.
  if (themesLoaded.length === 1) {
    const [active] = themesLoaded;
    const other = active === "aura" ? "lumo" : "aura";

    // Tokens from a theme other than the active one will not resolve.
    if (tokens[other].length > 0) {
      findings.push(
        finding(
          "warning",
          "MISMATCHED_THEME_TOKENS",
          `The ${active} theme is loaded, but --${other}-* CSS custom properties are used. ` +
            `These will not resolve unless the ${other} theme is also loaded.`,
          { confidence: "medium", evidence: tokens[other] }
        )
      );
    }

    // LumoUtility emits Lumo-specific CSS utility classes that Aura lacks.
    if (active === "aura" && lumoUtility.length > 0) {
      findings.push(
        finding(
          "error",
          "LUMO_UTILITY_WITHOUT_LUMO_THEME",
          "LumoUtility is used while the Aura theme is loaded. LumoUtility emits " +
            "Lumo-specific CSS utility classes that Aura does not define.",
          { confidence: "high", evidence: lumoUtility }
        )
      );
    }
  } else if (themesLoaded.length === 0 && (lumoUtility.length > 0 || tokenPrefixesUsed.length > 0)) {
    // Theme-dependent usage exists but no base theme is explicitly loaded, so we
    // cannot confidently say whether Aura, Lumo, or a custom theme is active.
    // Surface this as info (no error) so it is clear the checks were skipped, not
    // that everything passed.
    findings.push(
      finding(
        "info",
        "THEME_INDETERMINATE",
        "No base theme is explicitly loaded via @StyleSheet or a theme @import, so " +
          "the active theme could not be determined (it may be a custom theme). " +
          "Theme-dependent checks were skipped.",
        { confidence: "low", evidence: [...lumoUtility, ...tokens.aura, ...tokens.lumo] }
      )
    );
  }

  // Informational: legacy @Theme alongside new @StyleSheet theming.
  if (legacyTheme.length > 0 && themesLoaded.length > 0) {
    findings.push(
      finding(
        "info",
        "LEGACY_THEME_ANNOTATION",
        "A legacy @Theme annotation is present alongside @StyleSheet-based theming. " +
          "Confirm this is intentional when upgrading from Vaadin 24.",
        { confidence: "low", evidence: legacyTheme }
      )
    );
  }

  const ok = !findings.some((f) => f.level === "error");

  return {
    ok,
    themesLoaded,
    tokenPrefixesUsed,
    filesScanned: files.length,
    findings,
  };
}
