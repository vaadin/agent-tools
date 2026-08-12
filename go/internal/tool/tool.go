// Package tool defines the contract every tool implements, kept separate from
// the CLI so tools and the CLI can both depend on it without an import cycle.
package tool

// Args is the parsed invocation passed to a tool's Run function. Positionals
// includes any tool-specific flags (e.g. "--overwrite"); each tool parses those
// itself. The global flags (--json, --help, --version) are consumed by the CLI.
type Args struct {
	Positionals []string
	JSON        bool
	Cwd         string
}

// Result is what a tool returns.
//
//   - UsageError, when non-empty, signals a usage error (exit code 2). The CLI
//     renders it as { "tool", "ok": false, "usageError" } and ignores the rest.
//   - Otherwise, in JSON mode the CLI emits { "tool": <name>, "ok": <OK>,
//     ...Payload } — Payload is a tool-specific, json-tagged struct whose fields
//     are spliced in after "ok" (mirroring the JS `{ tool, ok, ...result }`).
//   - Human is the plain-text rendering used when --json is not set.
type Result struct {
	OK         bool
	UsageError string
	Payload    any
	Human      string
}

// Descriptor registers a tool with the CLI.
type Descriptor struct {
	Name    string
	Summary string
	Usage   string
	Run     func(Args) Result
}
