/**
 * Shared shape for a single finding produced by a tool.
 *
 * level: "error" | "warning" | "info"
 * code:  stable machine-readable identifier, e.g. "MULTIPLE_BASE_THEMES"
 * message: human-readable one-liner
 * confidence: "high" | "medium" | "low" — how sure the heuristic is
 * evidence: array of { file, line, snippet }
 */
export function finding(level, code, message, { confidence = "high", evidence = [] } = {}) {
  return { level, code, message, confidence, evidence };
}

export function evidence(file, line, snippet) {
  return { file, line, snippet: snippet.trim() };
}
