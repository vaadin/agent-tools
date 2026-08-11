import { test } from "node:test";
import assert from "node:assert/strict";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { run } from "../src/tools/check-theme-mixing.js";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const fixtures = path.join(__dirname, "fixtures");

test("flags a project that loads both Aura and Lumo", () => {
  const result = run({ positionals: ["mixed"], cwd: fixtures });
  assert.equal(result.ok, false);
  assert.deepEqual(result.themesLoaded.sort(), ["aura", "lumo"]);
  const error = result.findings.find((f) => f.code === "MULTIPLE_BASE_THEMES");
  assert.ok(error, "expected MULTIPLE_BASE_THEMES finding");
  assert.equal(error.level, "error");
});

test("passes a project that loads only one theme", () => {
  const result = run({ positionals: ["clean"], cwd: fixtures });
  assert.equal(result.ok, true);
  assert.deepEqual(result.themesLoaded, ["aura"]);
  assert.equal(result.findings.length, 0);
});

test("warns on mismatched theme tokens without failing the build", () => {
  const result = run({ positionals: ["mismatch"], cwd: fixtures });
  assert.equal(result.ok, true); // warning only, no error
  const warn = result.findings.find((f) => f.code === "MISMATCHED_THEME_TOKENS");
  assert.ok(warn, "expected MISMATCHED_THEME_TOKENS finding");
  assert.equal(warn.level, "warning");
});

test("flags LumoUtility usage while the Aura theme is loaded", () => {
  const result = run({ positionals: ["lumo-utility"], cwd: fixtures });
  assert.equal(result.ok, false);
  const error = result.findings.find(
    (f) => f.code === "LUMO_UTILITY_WITHOUT_LUMO_THEME"
  );
  assert.ok(error, "expected LUMO_UTILITY_WITHOUT_LUMO_THEME finding");
  assert.equal(error.level, "error");
  assert.equal(error.confidence, "high");
});

test("does not flag LumoUtility when the Lumo theme is loaded", () => {
  const result = run({ positionals: ["lumo-utility-ok"], cwd: fixtures });
  const error = result.findings.find(
    (f) => f.code === "LUMO_UTILITY_WITHOUT_LUMO_THEME"
  );
  assert.equal(error, undefined, "LumoUtility is valid under the Lumo theme");
});

test("reports a usage error for a missing directory", () => {
  const result = run({ positionals: ["does-not-exist"], cwd: fixtures });
  assert.ok(result.usageError);
});
