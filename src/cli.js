#!/usr/bin/env node
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";
import * as checkThemeMixing from "./tools/check-theme-mixing.js";

// --- tool registry ----------------------------------------------------------
// To add a tool: create src/tools/<name>.js exporting { name, summary, usage, run }
// and register it here.
const TOOLS = [checkThemeMixing];
const TOOL_BY_NAME = new Map(TOOLS.map((t) => [t.name, t]));

const __dirname = path.dirname(fileURLToPath(import.meta.url));

function pkgVersion() {
  try {
    const pkg = JSON.parse(
      readFileSync(path.join(__dirname, "..", "package.json"), "utf8")
    );
    return pkg.version;
  } catch {
    return "unknown";
  }
}

function parseArgv(argv) {
  const flags = { json: false, help: false, version: false };
  const positionals = [];
  for (const arg of argv) {
    switch (arg) {
      case "--json":
        flags.json = true;
        break;
      case "--help":
      case "-h":
        flags.help = true;
        break;
      case "--version":
      case "-v":
        flags.version = true;
        break;
      default:
        positionals.push(arg);
    }
  }
  return { flags, positionals };
}

function topLevelHelp() {
  const lines = [
    "vaadin-agent-tools — Vaadin tools for AI agents and humans",
    "",
    "Usage:",
    "  npx @vaadin/agent-tools <tool> [args] [--json]",
    "",
    "Tools:",
    ...TOOLS.map((t) => `  ${t.name.padEnd(22)} ${t.summary}`),
    "",
    "Global flags:",
    "  --json        Emit machine-readable JSON (recommended for agents)",
    "  -h, --help    Show help (use with a tool for tool-specific help)",
    "  -v, --version Print version",
  ];
  return lines.join("\n");
}

function renderHuman(toolName, result) {
  const out = [];
  out.push(`# ${toolName}`);
  if (result.themesLoaded) {
    out.push(
      `themes loaded: ${result.themesLoaded.length ? result.themesLoaded.join(", ") : "(none detected)"}`
    );
  }
  if (typeof result.filesScanned === "number") {
    out.push(`files scanned: ${result.filesScanned}`);
  }
  out.push("");
  if (!result.findings || result.findings.length === 0) {
    out.push("✓ No issues found.");
  } else {
    for (const f of result.findings) {
      const marker = f.level === "error" ? "✗" : f.level === "warning" ? "⚠" : "ℹ";
      out.push(`${marker} [${f.level}] ${f.code} (confidence: ${f.confidence})`);
      out.push(`  ${f.message}`);
      for (const e of f.evidence ?? []) {
        out.push(`    ${e.file}:${e.line}  ${e.snippet}`);
      }
      out.push("");
    }
  }
  return out.join("\n").trimEnd();
}

function main() {
  const { flags, positionals } = parseArgv(process.argv.slice(2));

  if (flags.version) {
    process.stdout.write(`${pkgVersion()}\n`);
    return 0;
  }

  const command = positionals.shift();

  if (!command || command === "help" || command === "list") {
    if (flags.json) {
      process.stdout.write(
        JSON.stringify(
          {
            version: pkgVersion(),
            tools: TOOLS.map((t) => ({ name: t.name, summary: t.summary })),
          },
          null,
          2
        ) + "\n"
      );
    } else {
      process.stdout.write(topLevelHelp() + "\n");
    }
    return 0;
  }

  const tool = TOOL_BY_NAME.get(command);
  if (!tool) {
    process.stderr.write(
      `Unknown tool: ${command}\n\n${topLevelHelp()}\n`
    );
    return 2;
  }

  if (flags.help) {
    process.stdout.write((tool.usage ?? tool.summary) + "\n");
    return 0;
  }

  const result = tool.run({ positionals, flags, cwd: process.cwd() });

  if (result.usageError) {
    if (flags.json) {
      process.stdout.write(
        JSON.stringify({ tool: tool.name, ok: false, usageError: result.usageError }, null, 2) + "\n"
      );
    } else {
      process.stderr.write(`Error: ${result.usageError}\n`);
    }
    return 2;
  }

  if (flags.json) {
    process.stdout.write(
      JSON.stringify({ tool: tool.name, ...result }, null, 2) + "\n"
    );
  } else {
    process.stdout.write(renderHuman(tool.name, result) + "\n");
  }

  return result.ok ? 0 : 1;
}

process.exit(main());
