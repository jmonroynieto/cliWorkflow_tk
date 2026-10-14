package bibstomp

import (
	"strings"
	"testing"
)

func TestDetexAccents(t *testing.T) {
	cases := []struct{ in, want string }{
		{`Mari\'{c}`, "Marić"},
		{`Kri\v{z}anovi\'{c}`, "Križanović"},
		{`Kre\v{s}imir`, "Krešimir"},
		{`\v{S}iki\'{c}`, "Šikić"},
		{`{\v S}iki\'{c}`, "Šikić"},
		{`M\"{u}ller`, "Müller"},
		{`M\"uller`, "Müller"},
		{`Fran\c{c}ois`, "François"},
		{`\k{A}dam`, "Ądam"},
		{`Erd\H{o}s`, "Erdős"},
		{`\r{A}ngstr\"om`, "Ångström"},
		{`Nu\~{n}ez`, "Nuñez"},
		{`G\={o}t\={o}`, "Gōtō"},
		{`\.{Z}ur`, "Żur"},
		{`Ba\u{g}c\i`, "Bağcı"},
		{`\'{\i}`, "í"},
	}
	for _, c := range cases {
		got, _ := Detex(c.in)
		if got != c.want {
			t.Errorf("Detex(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDetexSymbols(t *testing.T) {
	cases := []struct{ in, want string }{
		{`Micha\l{}`, "Michał"},
		{`Micha\l{} and Ola`, "Michał and Ola"}, // {} must not eat the space
		{`Micha\l\ Kowalski`, "Michał Kowalski"},
		{`{\O}stergaard`, "Østergaard"},
		{`S{\o}ren`, "Søren"},
		{`Kj{\ae}rgaard`, "Kjærgaard"},
		{`\TH{}\'{o}r\dh{}ur`, "Þórður"},
		{`Stra\ss{}e`, "Straße"},
		{`\DJ{}okovi\'{c}`, "Đoković"},
		{`\OE{}uvre`, "Œuvre"},
	}
	for _, c := range cases {
		got, _ := Detex(c.in)
		if got != c.want {
			t.Errorf("Detex(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDetexUnknownMacroReported(t *testing.T) {
	got, warns := Detex(`Smith\weirdmacro{X}`)
	if got != "SmithX" {
		t.Errorf("got %q", got)
	}
	if len(warns) != 1 || warns[0] != `\weirdmacro` {
		t.Errorf("warns = %v", warns)
	}
}

func TestDetexIdempotentOnPlainText(t *testing.T) {
	in := "Nagarajan, Niranjan"
	got, w := Detex(in)
	if got != in || w != nil {
		t.Errorf("Detex(%q) = %q, %v", in, got, w)
	}
}

// The two classes of non-ASCII letter, and why one pass is never enough.
func TestFoldDecomposable(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Marić", "Maric"},
		{"Križanović", "Krizanovic"},
		{"Müller", "Muller"},
		{"José", "Jose"},
		{"Gōtō", "Goto"},
		{"Ångström", "Angstrom"},
		{"Ądam", "Adam"},
		{"Erdős", "Erdos"},
	}
	for _, c := range cases {
		got, res := Fold(c.in)
		if got != c.want || len(res) != 0 {
			t.Errorf("Fold(%q) = %q, %v; want %q", c.in, got, res, c.want)
		}
	}
}

func TestFoldAtomic(t *testing.T) {
	// These survive NFD untouched -- no combining mark to strip.
	cases := []struct{ in, want string }{
		{"Michał", "Michal"},
		{"Søren", "Soren"},
		{"Kjærgaard", "Kjaergaard"},
		{"Þórðardóttir", "Thordardottir"},
		{"Straße", "Strasse"},
		{"Đoković", "Dokovic"},
		{"Bağcı", "Bagci"},
		{"Œuvre", "Oeuvre"},
		{"Ŋom", "Ngom"},
	}
	for _, c := range cases {
		got, res := Fold(c.in)
		if got != c.want || len(res) != 0 {
			t.Errorf("Fold(%q) = %q, residue %q; want %q", c.in, got, string(res), c.want)
		}
	}
}

func TestFoldReportsResidue(t *testing.T) {
	got, res := Fold("Ерофеев")
	if got != "" {
		t.Errorf("expected empty ASCII, got %q", got)
	}
	if len(res) == 0 {
		t.Error("expected residue runes to be reported, not silently dropped")
	}
}

func TestSplitAuthors(t *testing.T) {
	in := `Mari\'{c}, Josip and Kri\v{z}anovi\'{c}, Kre\v{s}imir and Riondet, Sylvain and Nagarajan, Niranjan and \v{S}iki\'{c}, Mile`
	got := SplitAuthors(in)
	if len(got) != 5 {
		t.Fatalf("got %d authors: %q", len(got), got)
	}
	if got[0] != `Mari\'{c}, Josip` {
		t.Errorf("first = %q", got[0])
	}
	if got[4] != `\v{S}iki\'{c}, Mile` {
		t.Errorf("last = %q", got[4])
	}
}

func TestSplitAuthorsBraceProtected(t *testing.T) {
	got := SplitAuthors(`{Smith and Wesson Ltd.} and Doe, Jane`)
	if len(got) != 2 {
		t.Fatalf("got %d: %q", len(got), got)
	}
	if got[0] != "{Smith and Wesson Ltd.}" {
		t.Errorf("corporate name was split: %q", got[0])
	}
}

func TestSplitAuthorsDropsOthers(t *testing.T) {
	got := SplitAuthors("Doe, Jane and others")
	if len(got) != 1 {
		t.Errorf("got %q", got)
	}
}

func TestParseNameForms(t *testing.T) {
	cases := []struct {
		in                   string
		first, von, last, jr string
	}{
		{"Mile Šikić", "Mile", "", "Šikić", ""},
		{"Šikić, Mile", "Mile", "", "Šikić", ""},
		{"van der Waals, Johannes", "Johannes", "van der", "Waals", ""},
		{"Johannes van der Waals", "Johannes", "van der", "Waals", ""},
		{"de la Cruz, María", "María", "de la", "Cruz", ""},
		{"Smith, Jr., John Q.", "John Q.", "", "Smith", "Jr."},
		{"Ludwig van Beethoven", "Ludwig", "van", "Beethoven", ""},
	}
	for _, c := range cases {
		n := ParseName(c.in)
		if n.First != c.first || n.von != c.von || n.Last != c.last || n.Jr != c.jr {
			t.Errorf("ParseName(%q) = {first:%q von:%q last:%q jr:%q}", c.in, n.First, n.von, n.Last, n.Jr)
		}
	}
}

func TestParseNameLiteral(t *testing.T) {
	n := ParseName("{World Health Organization}")
	if !n.Literal || n.Last != "World Health Organization" {
		t.Errorf("got %+v", n)
	}
}

// The headline case: a slug pass that runs before the TeX decode turns
// Mari\'{c} into "Mari" plus two dropped punctuation chars, yielding
// "Mari__2024". Decoding first gives Maric_2024.
func TestFamilyASCIIFixesTheBrokenKey(t *testing.T) {
	n := ParseName(`Mari\'{c}, Josip`)
	fam, residue, warns := n.FamilyASCII(false)
	if fam != "Maric" {
		t.Errorf("family = %q, want Maric", fam)
	}
	if len(residue) != 0 || len(warns) != 0 {
		t.Errorf("unexpected residue %q warns %v", string(residue), warns)
	}
}

func TestSanitiseKeyCollapsesSeparatorRuns(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Mari__2024", "Mari_2024"},
		{"__2020", "2020"},
		{"Mu_ller_2019", "Mu_ller_2019"},
		{"A---B", "A_B"},
		{"_lead_and_trail_", "lead_and_trail"},
	}
	for _, c := range cases {
		if got := sanitiseKey(c.in); got != c.want {
			t.Errorf("sanitiseKey(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

const sampleBib = `@article{Mari__2024,
  year = 2024,
  journal = {Bioinformatics},
  author = {Mari\'{c}, Josip and Kri\v{z}anovi\'{c}, Kre\v{s}imir and \v{S}iki\'{c}, Mile},
  title = {Comparative analysis of metagenomic classifiers},
  pages = {1--10}
}
`

func TestParseBibOffsets(t *testing.T) {
	entries, warns := ParseBib([]byte(sampleBib))
	if len(warns) != 0 {
		t.Fatalf("warns: %v", warns)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries", len(entries))
	}
	e := entries[0]
	if e.Type != "article" || e.Key != "Mari__2024" {
		t.Errorf("type=%q key=%q", e.Type, e.Key)
	}
	if sampleBib[e.KeyStart:e.KeyEnd] != "Mari__2024" {
		t.Errorf("key offsets point at %q", sampleBib[e.KeyStart:e.KeyEnd])
	}
	if got := e.FieldValue("year"); got != "2024" {
		t.Errorf("year = %q", got)
	}
	if got := e.FieldValue("pages"); got != "1--10" {
		t.Errorf("pages = %q", got)
	}
	if !strings.Contains(e.FieldValue("author"), `\v{S}iki`) {
		t.Errorf("author = %q", e.FieldValue("author"))
	}
}

func TestMintKeyEndToEnd(t *testing.T) {
	entries, _ := ParseBib([]byte(sampleBib))
	key, diags := MintKey(entries[0], DefaultKeyOpts())
	if key != "Maric_2024" {
		t.Errorf("key = %q, want Maric_2024", key)
	}
	if len(diags) != 0 {
		t.Errorf("diags = %+v", diags)
	}
}

func TestMintKeyTemplates(t *testing.T) {
	entries, _ := ParseBib([]byte(sampleBib))
	e := entries[0]
	cases := []struct {
		tmpl string
		n    int
		want string
	}{
		{"{authors}_{year}", 1, "Maric_2024"},
		{"{authors}_{year}", 2, "Maric_Krizanovic_2024"},
		{"{authors}{year}{title}", 3, "Maric_Krizanovic_Sikic2024Comparative"},
		{"{author}{shortyear}", 1, "Maric24"},
		{"{author}_{year}_{journal}", 1, "Maric_2024_Bioinformatics"},
	}
	for _, c := range cases {
		o := DefaultKeyOpts()
		o.Template, o.MaxAuthor = c.tmpl, c.n
		got, _ := MintKey(e, o)
		if got != c.want {
			t.Errorf("template %q n=%d -> %q, want %q", c.tmpl, c.n, got, c.want)
		}
	}
}

func TestSpliceIsMinimalDiff(t *testing.T) {
	src := []byte(sampleBib)
	entries, _ := ParseBib(src)
	e := entries[0]
	out := ApplySplices(src, []Splice{{e.KeyStart, e.KeyEnd, "Maric_2024"}})
	want := strings.Replace(sampleBib, "Mari__2024", "Maric_2024", 1)
	if string(out) != want {
		t.Errorf("splice altered more than the key:\n%s", string(out))
	}
}

func TestApplySplicesMultiple(t *testing.T) {
	src := []byte("abcdefghij")
	out := ApplySplices(src, []Splice{{5, 7, "XY"}, {1, 3, "Z"}})
	if string(out) != "aZdeXYhij" {
		t.Errorf("got %q", string(out))
	}
}

func TestKeyIsClean(t *testing.T) {
	for _, c := range []struct {
		k    string
		want bool
	}{
		{"Maric_2024", true},
		{"Mari__2024", false},
		{"Šikić2024", false},
		{"", false},
	} {
		if got := KeyIsClean(c.k); got != c.want {
			t.Errorf("KeyIsClean(%q) = %v", c.k, got)
		}
	}
}

func TestFirstContentWordSkipsStopwords(t *testing.T) {
	if got := firstContentWord("The analysis of a novel genome"); got != "analysis" {
		t.Errorf("got %q", got)
	}
	if got := firstContentWord("On the {ONT} basecalling problem"); got != "ONT" {
		t.Errorf("got %q", got)
	}
}

func TestExtractYearFromBiblatexDate(t *testing.T) {
	entries, _ := ParseBib([]byte(`@article{x, date = {2021-05-03}, author={Doe, J}}`))
	if got := extractYear(entries[0]); got != "2021" {
		t.Errorf("got %q", got)
	}
}

func TestSlugDropsPunctuation(t *testing.T) {
	got, _ := Slug("O'Brien-Smith", false)
	if got != "OBrienSmith" {
		t.Errorf("got %q", got)
	}
	got, _ = Slug("O'Brien-Smith", true)
	if got != "OBrien-Smith" {
		t.Errorf("got %q", got)
	}
}

// Duplicate records must mint the same key. The tool reports that; it does
// not invent a suffix to paper over it.
func TestDuplicateRecordsMintIdenticalKeys(t *testing.T) {
	src := `@article{a, author={S{\o}rensen, {\O}ystein}, year={2024}, title={Multilevel Semiparametric Latent Variable Modeling}}
@article{b, author={S{\o}rensen, {\O}ystein}, year={2024}, title={Multilevel Semiparametric Latent Variable Modeling}}`
	entries, _ := ParseBib([]byte(src))
	o := DefaultKeyOpts()
	k0, _ := MintKey(entries[0], o)
	k1, _ := MintKey(entries[1], o)
	if k0 != "Sorensen_2024" || k1 != "Sorensen_2024" {
		t.Fatalf("keys = %q, %q; want both Sorensen_2024", k0, k1)
	}
}

// {title} is still available explicitly, it is just no longer applied behind
// the user's back on collision.
func TestTitleIsOptInOnly(t *testing.T) {
	src := `@inbook{a, author={Holub, Jan}, year={2021}, title={Lyndon words and their applications}}`
	entries, _ := ParseBib([]byte(src))
	o := DefaultKeyOpts()
	if k, _ := MintKey(entries[0], o); k != "Holub_2021" {
		t.Errorf("default gave %q", k)
	}
	o.Template = "{author}_{year}_{title}"
	if k, _ := MintKey(entries[0], o); k != "Holub_2021_Lyndon" {
		t.Errorf("explicit template gave %q", k)
	}
}
