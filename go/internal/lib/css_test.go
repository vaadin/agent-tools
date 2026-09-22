package lib

import "testing"

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
