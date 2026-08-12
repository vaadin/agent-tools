package lib

import (
	"bytes"
	"encoding/json"
	"strings"
)

// MarshalIndentNoEscape marshals v with two-space indentation and without Go's
// default HTML escaping of <, > and &, so source-code snippets in evidence stay
// readable. Mirrors the CLI's jsonIndent so hook output matches --json output.
func MarshalIndentNoEscape(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return ""
	}
	return strings.TrimRight(buf.String(), "\n")
}
