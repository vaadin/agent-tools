package lib

import "strings"

// Finding is the shared shape for a single finding produced by a tool.
// Mirrors src/lib/findings.js.
//
//	Level:      "error" | "warning" | "info"
//	Code:       stable machine-readable identifier, e.g. "MULTIPLE_BASE_THEMES"
//	Message:    human-readable one-liner
//	Confidence: "high" | "medium" | "low" — how sure the heuristic is
//	Evidence:   the file:line snippets that triggered the finding
type Finding struct {
	Level      string     `json:"level"`
	Code       string     `json:"code"`
	Message    string     `json:"message"`
	Confidence string     `json:"confidence"`
	Evidence   []Evidence `json:"evidence"`
}

type Evidence struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Snippet string `json:"snippet"`
}

// NewFinding builds a Finding, defaulting confidence to "high" and normalizing a
// nil evidence slice to an empty one so it marshals as [] rather than null.
func NewFinding(level, code, message, confidence string, evidence []Evidence) Finding {
	if confidence == "" {
		confidence = "high"
	}
	if evidence == nil {
		evidence = []Evidence{}
	}
	return Finding{Level: level, Code: code, Message: message, Confidence: confidence, Evidence: evidence}
}

// NewEvidence builds an Evidence entry, trimming surrounding whitespace from the
// snippet (matching the JS evidence() helper).
func NewEvidence(file string, line int, snippet string) Evidence {
	return Evidence{File: file, Line: line, Snippet: strings.TrimSpace(snippet)}
}
