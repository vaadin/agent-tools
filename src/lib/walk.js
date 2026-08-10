import fs from "node:fs";
import path from "node:path";

const DEFAULT_IGNORE = new Set([
  "node_modules",
  "target",
  "build",
  "dist",
  "out",
  ".git",
  ".idea",
  ".vscode",
  ".mvn",
  "frontend-generated",
]);

/**
 * Recursively walk `root`, yielding absolute file paths whose basename matches
 * one of the provided extensions. Directories in the ignore set are skipped.
 *
 * @param {string} root
 * @param {object} [options]
 * @param {string[]} [options.extensions] e.g. [".java", ".css"]
 * @param {Set<string>} [options.ignore]
 * @returns {string[]}
 */
export function walk(root, { extensions, ignore = DEFAULT_IGNORE } = {}) {
  const results = [];
  const exts = extensions ? new Set(extensions.map((e) => e.toLowerCase())) : null;

  const stack = [root];
  while (stack.length > 0) {
    const dir = stack.pop();
    let entries;
    try {
      entries = fs.readdirSync(dir, { withFileTypes: true });
    } catch {
      continue; // unreadable directory — skip
    }
    for (const entry of entries) {
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        if (!ignore.has(entry.name)) stack.push(full);
      } else if (entry.isFile()) {
        if (!exts || exts.has(path.extname(entry.name).toLowerCase())) {
          results.push(full);
        }
      }
    }
  }
  return results.sort();
}
