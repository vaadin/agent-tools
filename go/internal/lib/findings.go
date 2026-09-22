package lib

import (
	"fmt"
	"strings"
)

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

// RenderFindings formats a finding list for the plain-text (non---json) output,
// returning one line per output row so a tool can splice it into its own header.
// Shared so every tool's human output reads the same.
func RenderFindings(findings []Finding) []string {
	if len(findings) == 0 {
		return []string{"✓ No issues found."}
	}
	var out []string
	for _, f := range findings {
		marker := "ℹ"
		switch f.Level {
		case "error":
			marker = "✗"
		case "warning":
			marker = "⚠"
		}
		out = append(out, fmt.Sprintf("%s [%s] %s (confidence: %s)", marker, f.Level, f.Code, f.Confidence))
		out = append(out, "  "+f.Message)
		for _, e := range f.Evidence {
			out = append(out, fmt.Sprintf("    %s:%d  %s", e.File, e.Line, e.Snippet))
		}
		out = append(out, "")
	}
	return out
}
