package lib

import "strings"

// A deliberately small CSS reader: enough structure to answer "which custom
// property is assigned, to what, and under which selector" without pulling in a
// real CSS parser. It understands comments, strings, parentheses, at-rules and
// nesting — which is all the checks need — and never fails on input it does not
// understand; unparseable regions simply yield no declarations.

// CSSDeclaration is one custom-property declaration found in a stylesheet.
// Selectors is the chain of enclosing rule preludes, outermost first, with
// at-rule preludes (@media, @supports, …) left out because they do not select
// elements. Offset is the byte offset of the property name in the source.
type CSSDeclaration struct {
	Property  string
	Value     string
	Offset    int
	Selectors []string
}

// CSSVarRead is one var(--name) reference found in a stylesheet.
type CSSVarRead struct {
	Name   string
	Offset int
}

// BlankComments replaces every /* … */ comment with spaces, preserving the
// length of the input (and the newlines inside the comment) so byte offsets into
// the result are still valid offsets into the original source.
//
// String literals are skipped, so a comment marker inside one — content: "/*" —
// does not start a comment; and quotes inside a comment do not start a string.
func BlankComments(css string) string {
	return blankComments(css, false)
}

// BlankJavaComments is BlankComments for Java sources: it also blanks // line
// comments, and skips char literals as well as strings. Offsets are preserved
// the same way.
func BlankJavaComments(src string) string {
	return blankComments(src, true)
}

// blankComments walks comments and string/char literals in one pass so the two
// cannot be confused for one another. lineComments enables // (Java only — in
// CSS a // sequence is ordinary text, as in url(https://example.com)).
func blankComments(src string, lineComments bool) string {
	out := []byte(src)
	blank := func(from, to int) {
		for k := from; k < to && k < len(out); k++ {
			// Line terminators are kept so line numbers computed against the
			// original source still hold. Java (JLS 3.4) and CSS both count a bare
			// CR as one, so neither may be blanked away.
			if out[k] != '\n' && out[k] != '\r' {
				out[k] = ' '
			}
		}
	}

	for i := 0; i < len(out); {
		c := out[i]

		switch {
		case c == '/' && i+1 < len(out) && out[i+1] == '*':
			j := i + 2
			for j+1 < len(out) && !(out[j] == '*' && out[j+1] == '/') {
				j++
			}
			end := j + 2
			if end > len(out) {
				end = len(out) // unterminated comment: blank to EOF
			}
			blank(i, end)
			i = end

		case lineComments && c == '/' && i+1 < len(out) && out[i+1] == '/':
			j := i
			for j < len(out) && out[j] != '\n' && out[j] != '\r' {
				j++
			}
			blank(i, j)
			i = j

		case c == '"' || c == '\'':
			// Step over the literal so its contents are never read as structure.
			j := i + 1
			for j < len(out) && out[j] != c {
				if out[j] == '\\' {
					j++
				}
				j++
			}
			i = j + 1

		default:
			i++
		}
	}
	return string(out)
}

// ParseCSS scans a stylesheet and returns its custom-property declarations
// (those whose property name starts with "--") and every var() reference in it.
// Pass the output of BlankComments so commented-out code is ignored; offsets
// remain valid against the original source.
func ParseCSS(css string) ([]CSSDeclaration, []CSSVarRead) {
	var decls []CSSDeclaration
	var reads []CSSVarRead
	var selectors []string // enclosing non-at-rule preludes, outermost first

	var buf strings.Builder
	bufStart := -1
	depth := 0      // parenthesis depth
	blockDepth := 0 // { } nesting

	flushDeclaration := func() {
		text := buf.String()
		buf.Reset()
		start := bufStart
		bufStart = -1
		if blockDepth == 0 || start < 0 {
			return
		}
		colon := strings.IndexByte(text, ':')
		if colon < 0 {
			return
		}
		prop := strings.TrimSpace(text[:colon])
		if !strings.HasPrefix(prop, "--") {
			return
		}
		decls = append(decls, CSSDeclaration{
			Property:  prop,
			Value:     strings.TrimSpace(text[colon+1:]),
			Offset:    start,
			Selectors: append([]string(nil), selectors...),
		})
	}

	for i := 0; i < len(css); i++ {
		c := css[i]

		// Record var(--name) here rather than in a separate pass, so a reference
		// that only looks like one inside a string literal is not counted: the
		// string branch below skips past those.
		if (c == 'v' || c == 'V') && hasPrefixFold(css[i:], "var(") &&
			(i == 0 || !isCSSIdentByte(css[i-1])) {
			j := i + 4
			for j < len(css) && (css[j] == ' ' || css[j] == '\t' || css[j] == '\n' || css[j] == '\r') {
				j++
			}
			if strings.HasPrefix(css[j:], "--") {
				k := j
				for k < len(css) && isCSSIdentByte(css[k]) {
					k++
				}
				reads = append(reads, CSSVarRead{Name: css[j:k], Offset: j})
			}
		}

		switch c {
		case '"', '\'':
			// Consume the whole string literal so its contents cannot be read as
			// structure. It stays in buf: a var() inside a string is not a read,
			// and a quoted value is still the declaration's value.
			quote := c
			j := i + 1
			for j < len(css) && css[j] != quote {
				if css[j] == '\\' {
					j++
				}
				j++
			}
			if j >= len(css) {
				j = len(css) - 1
			}
			if bufStart < 0 {
				bufStart = i
			}
			buf.WriteString(css[i : j+1])
			i = j
			continue

		case '(':
			depth++

		case ')':
			if depth > 0 {
				depth--
			}

		case '{':
			if depth == 0 {
				prelude := strings.TrimSpace(buf.String())
				buf.Reset()
				bufStart = -1
				blockDepth++
				if prelude != "" && !strings.HasPrefix(prelude, "@") {
					selectors = append(selectors, prelude)
				} else {
					// Mark the level as an at-rule (or an unreadable prelude) so the
					// matching '}' pops nothing.
					selectors = append(selectors, "")
				}
				continue
			}

		case '}':
			if depth == 0 {
				flushDeclaration() // a last declaration may omit its semicolon
				if blockDepth > 0 {
					blockDepth--
					selectors = selectors[:len(selectors)-1]
				}
				continue
			}

		case ';':
			if depth == 0 {
				flushDeclaration()
				continue
			}
		}

		if bufStart < 0 && c != ' ' && c != '\t' && c != '\n' && c != '\r' && c != '\f' {
			bufStart = i
		}
		if bufStart >= 0 {
			buf.WriteByte(c)
		}
	}

	// Drop the empty placeholders pushed for at-rule levels.
	for i := range decls {
		kept := decls[i].Selectors[:0]
		for _, s := range decls[i].Selectors {
			if s != "" {
				kept = append(kept, s)
			}
		}
		decls[i].Selectors = kept
	}

	return decls, reads
}

// hasPrefixFold reports whether s starts with the ASCII-lowercase prefix,
// ignoring case — CSS function names are case-insensitive.
func hasPrefixFold(s, lowerPrefix string) bool {
	if len(s) < len(lowerPrefix) {
		return false
	}
	for i := 0; i < len(lowerPrefix); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != lowerPrefix[i] {
			return false
		}
	}
	return true
}

// isCSSIdentByte reports whether b can appear in a CSS identifier. Every byte at
// or above 0x80 counts: CSS identifiers may contain non-ASCII characters, and
// treating a UTF-8 lead or continuation byte as a boundary would both truncate
// a custom property name and let one class name match a longer one.
func isCSSIdentByte(b byte) bool {
	return b == '-' || b == '_' || b >= 0x80 ||
		(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// SelectorChainMatches reports whether any selector in chain mentions any of the
// given tokens. A token made of identifier characters at its edges must match on
// an identifier boundary, so "vaadin-tab" does not match "vaadin-tabs"; a token
// like "::part(overlay)" matches anywhere, so it also matches
// "vaadin-dialog::part(overlay)".
func SelectorChainMatches(chain []string, tokens []string) bool {
	normalized := make([]string, len(tokens))
	for i, tok := range tokens {
		normalized[i] = normalizeSelector(tok)
	}
	for _, sel := range chain {
		hay := normalizeSelector(sel)
		for _, tok := range normalized {
			if containsSelectorToken(hay, tok) {
				return true
			}
		}
	}
	return false
}

// normalizeSelector folds away the spellings CSS treats as equivalent, so a
// token written one way still matches a selector written another: case,
// "double" vs 'single' attribute quotes, and whitespace inside an attribute
// selector ([theme~= "success"] == [theme~='success']).
//
// Whitespace OUTSIDE brackets is left alone: there it is the descendant
// combinator, and collapsing it would weld two identifiers into one, so
// "vaadin-notification-card vaadin-button" would stop mentioning vaadin-button.
func normalizeSelector(sel string) string {
	var b strings.Builder
	b.Grow(len(sel))
	inBrackets := 0
	var quote byte // the open string delimiter, or 0 outside a string
	for i := 0; i < len(sel); i++ {
		c := sel[i]

		// Inside a quoted value nothing is structure: a '[' there does not open a
		// bracket, a space there is part of the value, and the case is significant
		// (attribute values are case-sensitive, unlike the rest of a selector).
		if quote != 0 {
			if c == '\\' && i+1 < len(sel) {
				b.WriteByte(c)
				i++
				b.WriteByte(sel[i])
				continue
			}
			if c == quote {
				quote = 0
				b.WriteByte('\'') // both delimiters normalize to '
				continue
			}
			b.WriteByte(c)
			continue
		}

		switch {
		case c == '"' || c == '\'':
			quote = c
			b.WriteByte('\'')
		case c == '[':
			inBrackets++
			b.WriteByte(c)
		case c == ']':
			if inBrackets > 0 {
				inBrackets--
			}
			b.WriteByte(c)
		case inBrackets > 0 && (c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f'):
			// dropped
		case c >= 'A' && c <= 'Z':
			b.WriteByte(c + 'a' - 'A')
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

func containsSelectorToken(haystack, token string) bool {
	if token == "" {
		return false
	}
	needLeft := isCSSIdentByte(token[0])
	needRight := isCSSIdentByte(token[len(token)-1])
	for from := 0; ; {
		idx := strings.Index(haystack[from:], token)
		if idx < 0 {
			return false
		}
		start := from + idx
		end := start + len(token)
		okLeft := !needLeft || start == 0 || !isCSSIdentByte(haystack[start-1])
		okRight := !needRight || end == len(haystack) || !isCSSIdentByte(haystack[end])
		if okLeft && okRight {
			return true
		}
		from = start + 1
	}
}
