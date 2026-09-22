package lib

import (
	"strings"
	"testing"
)

func parse(t *testing.T, css string) ([]CSSDeclaration, []CSSVarRead) {
	t.Helper()
	return ParseCSS(BlankComments(css))
}

func TestBlankCommentsPreservesOffsets(t *testing.T) {
	in := "a{/* x\ny */--p: 1}"
	out := BlankComments(in)
	if len(out) != len(in) {
		t.Fatalf("length changed: %d -> %d", len(in), len(out))
	}
	if want := "a{    \n    --p: 1}"; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func TestParseCSSCapturesSelectorChain(t *testing.T) {
	decls, _ := parse(t, `
@media (min-width: 900px) {
  .card {
    --a: 1px;
    &:hover { --b: 2px }
  }
}`)
	if len(decls) != 2 {
		t.Fatalf("declarations = %d, want 2: %+v", len(decls), decls)
	}
	if got := decls[0].Selectors; len(got) != 1 || got[0] != ".card" {
		t.Fatalf("selectors = %v, want [.card] (the @media prelude is not a selector)", got)
	}
	if got := decls[1].Selectors; len(got) != 2 || got[0] != ".card" || got[1] != "&:hover" {
		t.Fatalf("nested selectors = %v, want [.card &:hover]", got)
	}
	// The last declaration in a block may omit its semicolon.
	if decls[1].Property != "--b" || decls[1].Value != "2px" {
		t.Fatalf("got %q: %q, want --b: 2px", decls[1].Property, decls[1].Value)
	}
}

func TestParseCSSIgnoresCommentsAndStrings(t *testing.T) {
	decls, reads := parse(t, `
.x {
  /* --commented: var(--gone); */
  content: "} --not-a-decl: var(--nope)";
  --real: var(--dep);
}`)
	if len(decls) != 1 || decls[0].Property != "--real" {
		t.Fatalf("declarations = %+v, want only --real", decls)
	}
	if len(reads) != 1 || reads[0].Name != "--dep" {
		t.Fatalf("reads = %+v, want only --dep", reads)
	}
}

func TestParseCSSHandlesFunctionsInValues(t *testing.T) {
	decls, reads := parse(t, `.x { --c: light-dark(var(--a), var(--b)); --d: 1 }`)
	if len(decls) != 2 {
		t.Fatalf("declarations = %d, want 2: %+v", len(decls), decls)
	}
	if decls[0].Value != "light-dark(var(--a), var(--b))" {
		t.Fatalf("value = %q", decls[0].Value)
	}
	if len(reads) != 2 || reads[0].Name != "--a" || reads[1].Name != "--b" {
		t.Fatalf("reads = %+v, want --a and --b", reads)
	}
}

func TestParseCSSSkipsTopLevelText(t *testing.T) {
	decls, _ := parse(t, "@import url(https://example.com/a.css);\n.x { --p: 1px }")
	if len(decls) != 1 || decls[0].Property != "--p" {
		t.Fatalf("declarations = %+v, want only --p", decls)
	}
}

func TestSelectorChainMatchesRespectsIdentifierBoundaries(t *testing.T) {
	cases := []struct {
		chain []string
		token string
		want  bool
	}{
		{[]string{"vaadin-tabs"}, "vaadin-tab", false},
		{[]string{"vaadin-tab"}, "vaadin-tab", true},
		{[]string{"vaadin-tab:hover"}, "vaadin-tab", true},
		{[]string{".aura-surface-solid"}, ".aura-surface", false},
		{[]string{".hero.aura-surface"}, ".aura-surface", true},
		{[]string{"vaadin-text-field::part(input-field)"}, "::part(input-field)", true},
		{[]string{"vaadin-dialog::part(overlay)"}, "::part(overlay)", true},
		{[]string{".my-card", "vaadin-card"}, "vaadin-card", true},
		{[]string{".my-card"}, "vaadin-card", false},
		{[]string{"VAADIN-CARD"}, "vaadin-card", true},
	}
	for _, c := range cases {
		if got := SelectorChainMatches(c.chain, []string{c.token}); got != c.want {
			t.Errorf("SelectorChainMatches(%v, %q) = %v, want %v", c.chain, c.token, got, c.want)
		}
	}
}

// A comment marker inside a string literal must not start a comment, and a
// quote inside a comment must not start a string.
func TestBlankCommentsDoesNotConfuseStringsAndComments(t *testing.T) {
	decls, _ := parse(t, `.x {
  content: "/*";
  --a: 0;
}
/* it's a comment with an apostrophe */
.y { --b: 1 }`)
	if len(decls) != 2 {
		t.Fatalf("declarations = %+v, want --a and --b", decls)
	}
	if decls[0].Property != "--a" || decls[1].Property != "--b" {
		t.Fatalf("got %q and %q", decls[0].Property, decls[1].Property)
	}
}

func TestBlankCommentsHandlesUnterminatedComment(t *testing.T) {
	if out := BlankComments("a{--p:1} /* never closed"); len(out) != len("a{--p:1} /* never closed") {
		t.Fatalf("length changed: %d", len(out))
	}
}

// CSS has no // comment: url(https://…) must survive blanking intact.
func TestBlankCommentsKeepsProtocolRelativeText(t *testing.T) {
	in := `@import url(https://example.com/a.css);` + "\n" + `.x { --p: 1px }`
	if out := BlankComments(in); out != in {
		t.Fatalf("blanking altered CSS with no comments:\n got %q\nwant %q", out, in)
	}
}

func TestBlankJavaCommentsBlanksLineAndBlockComments(t *testing.T) {
	in := "// @StyleSheet(Aura.STYLESHEET)\n/* @StyleSheet(Lumo.STYLESHEET) */\nString s = \"// not a comment\";"
	out := BlankJavaComments(in)
	if len(out) != len(in) {
		t.Fatalf("length changed: %d -> %d", len(in), len(out))
	}
	if strings.Contains(out, "Aura") || strings.Contains(out, "Lumo") {
		t.Fatalf("commented annotations survived: %q", out)
	}
	if !strings.Contains(out, "// not a comment") {
		t.Fatalf("a comment marker inside a string literal was blanked: %q", out)
	}
}

// CSS identifiers may contain non-ASCII characters.
func TestParseCSSHandlesNonASCIIIdentifiers(t *testing.T) {
	decls, reads := parse(t, ".x { --aura-card-é: red; color: var(--aura-card-é); }")
	if len(decls) != 1 || decls[0].Property != "--aura-card-é" {
		t.Fatalf("declarations = %+v, want --aura-card-é", decls)
	}
	if len(reads) != 1 || reads[0].Name != "--aura-card-é" {
		t.Fatalf("reads = %+v, want --aura-card-é (not truncated)", reads)
	}
}

func TestSelectorChainMatchesNormalizesEquivalentSpellings(t *testing.T) {
	cases := []struct {
		chain []string
		token string
		want  bool
	}{
		{[]string{`[theme~="success"]`}, `[theme~='success']`, true},
		{[]string{`[theme~= "success"]`}, `[theme~='success']`, true},
		// A selector is matched case-insensitively, but an attribute VALUE is
		// case-sensitive in CSS, so these two are genuinely different selectors.
		{[]string{`[THEME~='SUCCESS']`}, `[theme~='success']`, false},
		{[]string{`[THEME~='success']`}, `[theme~='success']`, true},
		{[]string{`[theme~='danger']`}, `[theme~='success']`, false},
		// A non-ASCII suffix makes it a different class identifier.
		{[]string{".aura-surfaceé"}, ".aura-surface", false},
	}
	for _, c := range cases {
		if got := SelectorChainMatches(c.chain, []string{c.token}); got != c.want {
			t.Errorf("SelectorChainMatches(%v, %q) = %v, want %v", c.chain, c.token, got, c.want)
		}
	}
}

// Whitespace outside brackets is the descendant combinator, and must keep
// acting as an identifier boundary rather than being collapsed away.
func TestSelectorChainMatchesKeepsDescendantCombinator(t *testing.T) {
	cases := []struct {
		chain []string
		token string
		want  bool
	}{
		{[]string{"vaadin-notification-card vaadin-button"}, "vaadin-button", true},
		{[]string{"vaadin-notification-card > vaadin-button"}, "vaadin-button", true},
		{[]string{".card vaadin-tab"}, "vaadin-tab", true},
		{[]string{".card vaadin-tabx"}, "vaadin-tab", false},
	}
	for _, c := range cases {
		if got := SelectorChainMatches(c.chain, []string{c.token}); got != c.want {
			t.Errorf("SelectorChainMatches(%v, %q) = %v, want %v", c.chain, c.token, got, c.want)
		}
	}
}

// A quoted attribute value is not structure: brackets in it must not shift the
// bracket depth, and whitespace in it must not be folded away.
func TestSelectorChainMatchesTreatsQuotedValuesAsOpaque(t *testing.T) {
	cases := []struct {
		chain []string
		token string
		want  bool
	}{
		// A '[' inside a value used to leave the depth positive, which folded the
		// descendant combinators after it and hid the real subject.
		{[]string{`[data-label="["] span vaadin-button`}, "vaadin-button", true},
		{[]string{`[data-label="]"] vaadin-card`}, "vaadin-card", true},
		// Folding whitespace inside a value used to forge a whitelisted class.
		{[]string{`[data-label=".aura- surface"]`}, ".aura-surface", false},
		{[]string{`[data-label="a\"b"] vaadin-card`}, "vaadin-card", true},
		// Whitespace outside the value is still insignificant.
		{[]string{`[theme~= "success"]`}, `[theme~='success']`, true},
	}
	for _, c := range cases {
		if got := SelectorChainMatches(c.chain, []string{c.token}); got != c.want {
			t.Errorf("SelectorChainMatches(%v, %q) = %v, want %v", c.chain, c.token, got, c.want)
		}
	}
}

// A Java line comment ends at any line terminator, and JLS 3.4 counts a bare CR
// as one. Line terminators are also preserved by blanking so offsets hold.
func TestBlankJavaCommentsEndsLineCommentAtCarriageReturn(t *testing.T) {
	in := "// comment\r@StyleSheet(Aura.STYLESHEET)\rclass App {}"
	out := BlankJavaComments(in)
	if len(out) != len(in) {
		t.Fatalf("length changed: %d -> %d", len(in), len(out))
	}
	if !strings.Contains(out, "@StyleSheet(Aura.STYLESHEET)") {
		t.Fatalf("code after a CR-terminated line comment was blanked: %q", out)
	}
	if strings.Contains(out, "comment") {
		t.Fatalf("the comment itself survived: %q", out)
	}
	if strings.Count(out, "\r") != strings.Count(in, "\r") {
		t.Fatalf("carriage returns were not preserved: %q", out)
	}
}

func TestBlankJavaTextBlocksBlanksOnlyTheContents(t *testing.T) {
	src := "String a = \"keep me\";\n" +
		"String doc = \"\"\"\n" +
		"    box.getStyle().set(\"--aura-background-color\", \"#fff\");\n" +
		"    \"\"\";\n" +
		"String b = \"keep me too\";\n"
	got := BlankJavaTextBlocks(src)

	if len(got) != len(src) {
		t.Fatalf("length changed: %d → %d", len(src), len(got))
	}
	if strings.Count(got, "\n") != strings.Count(src, "\n") {
		t.Fatal("newlines must survive so line numbers still hold")
	}
	if strings.Contains(got, "--aura-background-color") {
		t.Errorf("text-block contents were not blanked: %q", got)
	}
	for _, keep := range []string{`"keep me"`, `"keep me too"`} {
		if !strings.Contains(got, keep) {
			t.Errorf("ordinary string literal %s was blanked away: %q", keep, got)
		}
	}
}

// A quote inside an ordinary literal must not look like the start of a block,
// and an empty string ("" — two quotes, not three) is not one either.
func TestBlankJavaTextBlocksSkipsOrdinaryLiterals(t *testing.T) {
	src := `String quote = "\"\"\""; String empty = ""; String tail = "--aura-red";`
	if got := BlankJavaTextBlocks(src); got != src {
		t.Fatalf("nothing should have been blanked:\n got %q\nwant %q", got, src)
	}
}

// An unterminated block blanks to EOF rather than looping or panicking.
func TestBlankJavaTextBlocksHandlesUnterminatedBlock(t *testing.T) {
	src := "String doc = \"\"\"\n  set(\"--aura-red-text\", \"#900\");\n"
	got := BlankJavaTextBlocks(src)
	if len(got) != len(src) {
		t.Fatalf("length changed: %d → %d", len(src), len(got))
	}
	if strings.Contains(got, "--aura-red-text") {
		t.Errorf("an unterminated block must be blanked to EOF: %q", got)
	}
}
