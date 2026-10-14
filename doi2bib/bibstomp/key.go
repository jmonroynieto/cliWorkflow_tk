package bibstomp

import (
	"regexp"
	"strings"
)

// KeyOpts controls cite key minting.
type KeyOpts struct {
	Template   string // e.g. "{authors}_{year}"
	MaxAuthor  int    // how many surnames {authors} may include
	Joiner     string // separator between surnames
	IncludeVon bool   // fold "van der" into the surname
	Case       string // "preserve", "lower", "upper", "title"
	Suffix     string // "alpha" (a,b,c) or "numeric" (_2,_3)
	EtAl       string // appended to {authors} when truncated, e.g. "EtAl"
}

func DefaultKeyOpts() KeyOpts {
	return KeyOpts{
		Template:  "{authors}_{year}",
		MaxAuthor: 1,
		Joiner:    "_",
		Case:      "preserve",
	}
}

var titleStop = map[string]bool{
	"a": true, "an": true, "the": true, "of": true, "on": true, "in": true,
	"for": true, "and": true, "or": true, "to": true, "with": true,
	"from": true, "by": true, "at": true, "as": true, "is": true, "are": true,
	"into": true, "via": true, "using": true, "towards": true, "toward": true,
	"new": true, "novel": true,
}

var yearRe = regexp.MustCompile(`\b(1[6-9]\d{2}|20\d{2}|21\d{2})\b`)

// Diagnostic records anything that needed a human's attention.
type Diagnostic struct {
	Key    string
	Kind   string // "residue", "macro", "no-author", "no-year", "collision"
	Detail string
}

// MintKey builds a cite key for an entry, returning diagnostics for anything
// it could not resolve cleanly.
func MintKey(e *Entry, o KeyOpts) (string, []Diagnostic) {
	var diags []Diagnostic

	authorField := e.FieldValue("author")
	if authorField == "" {
		authorField = e.FieldValue("editor")
	}
	names := SplitAuthors(authorField)

	var surnames []string
	for _, raw := range names {
		n := ParseName(raw)
		fam, residue, warns := n.FamilyASCII(o.IncludeVon)
		if len(residue) > 0 {
			diags = append(diags, Diagnostic{
				e.Key, "residue",
				"unmappable runes in surname " + string(residue) + " (from " + raw + ")",
			})
		}
		for _, w := range warns {
			diags = append(diags, Diagnostic{e.Key, "macro", "unknown TeX macro " + w + " in " + raw})
		}
		// Keep scanning past MaxAuthor: a broken 5th author still means the
		// entry needs review even though it never reaches the key.
		if fam != "" && (o.MaxAuthor <= 0 || len(surnames) < o.MaxAuthor) {
			surnames = append(surnames, fam)
		}
	}
	if len(surnames) == 0 {
		diags = append(diags, Diagnostic{e.Key, "no-author", "no usable surname"})
		surnames = []string{"Anon"}
	}

	joiner := o.Joiner
	if joiner == "" {
		joiner = "_"
	}
	authors := strings.Join(surnames, joiner)
	if o.EtAl != "" && len(names) > len(surnames) {
		authors += o.EtAl
	}

	year := extractYear(e)
	if year == "" {
		diags = append(diags, Diagnostic{e.Key, "no-year", "no year or date field"})
		year = "0000"
	}

	title, _ := Detex(e.FieldValue("title"))
	titleWord := firstContentWord(title)

	journal, _ := Detex(e.FieldValue("journal"))
	if journal == "" {
		journal, _ = Detex(e.FieldValue("booktitle"))
	}
	journalSlug, _ := Slug(journal, false)

	doiSlug, _ := Slug(strings.ReplaceAll(e.FieldValue("doi"), "/", "_"), false)

	r := strings.NewReplacer(
		"{authors}", authors,
		"{author}", surnames[0],
		"{year}", year,
		"{shortyear}", shortYear(year),
		"{title}", titleWord,
		"{journal}", journalSlug,
		"{doi}", doiSlug,
		"{type}", e.Type,
	)
	key := r.Replace(o.Template)
	key = sanitiseKey(key)

	switch o.Case {
	case "lower":
		key = strings.ToLower(key)
	case "upper":
		key = strings.ToUpper(key)
	}
	return key, diags
}

func extractYear(e *Entry) string {
	for _, f := range []string{"year", "date", "issued", "urldate"} {
		v := e.FieldValue(f)
		if m := yearRe.FindString(v); m != "" {
			return m
		}
	}
	return ""
}

func shortYear(y string) string {
	if len(y) == 4 {
		return y[2:]
	}
	return y
}

func firstContentWord(title string) string {
	fields := strings.FieldsFunc(title, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r > 127)
	})
	for _, w := range fields {
		lw := strings.ToLower(w)
		if titleStop[lw] || len(w) < 3 {
			continue
		}
		s, _ := Slug(w, false)
		if s != "" {
			return s
		}
	}
	if len(fields) > 0 {
		s, _ := Slug(fields[0], false)
		return s
	}
	return ""
}

// sanitiseKey collapses the separator runs that empty substitutions leave
// behind -- the exact failure mode that turns Mari\'{c} into "Mari__2024".
var sepRun = regexp.MustCompile(`[_:.-]{2,}`)

func sanitiseKey(k string) string {
	var b strings.Builder
	for _, r := range k {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '_', r == '-', r == ':', r == '.':
			b.WriteRune(r)
		}
	}
	k = sepRun.ReplaceAllString(b.String(), "_")
	return strings.Trim(k, "_-:.")
}

// KeyIsClean reports whether an existing key is already ASCII and free of the
// separator runs left by a naive slug pass.
func KeyIsClean(k string) bool {
	if k == "" {
		return false
	}
	for _, r := range k {
		if r > 127 {
			return false
		}
	}
	return !sepRun.MatchString(k)
}
