package bibstomp

import (
	"strings"
	"unicode"
)

// Name is a parsed BibTeX personal name.
type Name struct {
	First   string
	von     string
	Last    string
	Jr      string
	Literal bool // brace-protected corporate name, e.g. {World Health Organization}
	Warns   []string
}

// SplitAuthors splits a BibTeX author/editor field on the top-level " and "
// separator. Braced groups are opaque, so {Smith and Wesson Ltd.} stays one
// name, and \and inside a macro is not a separator.
func SplitAuthors(field string) []string {
	var out []string
	r := []rune(field)
	depth := 0
	start := 0
	for i := 0; i < len(r); i++ {
		switch r[i] {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case '\\':
			i++ // escaped char, skip
		case 'a', 'A':
			if depth != 0 || i == 0 {
				continue
			}
			if !isSpace(r[i-1]) {
				continue
			}
			if i+3 > len(r) {
				continue
			}
			if !strings.EqualFold(string(r[i:i+3]), "and") {
				continue
			}
			if i+3 < len(r) && !isSpace(r[i+3]) {
				continue
			}
			out = append(out, strings.TrimSpace(string(r[start:i])))
			i += 3
			start = i
		}
	}
	if tail := strings.TrimSpace(string(r[start:])); tail != "" {
		out = append(out, tail)
	}
	var clean []string
	for _, s := range out {
		if s != "" && !strings.EqualFold(s, "others") {
			clean = append(clean, s)
		}
	}
	return clean
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

// ParseName implements the three BibTeX name forms:
//
//	"First von Last"
//	"von Last, First"
//	"von Last, Jr, First"
//
// A von part is a run of tokens whose first alphabetic character (looking
// through braces and TeX accents) is lowercase.
func ParseName(s string) Name {
	s = strings.TrimSpace(s)
	var n Name

	// A wholly brace-wrapped name is a literal / corporate name.
	if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") && matchedOuter(s) {
		inner := s[1 : len(s)-1]
		if !strings.Contains(inner, ",") {
			n.Last = inner
			n.Literal = true
			return n
		}
	}

	parts := splitTopLevel(s, ',')
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}

	switch len(parts) {
	case 1:
		toks := splitTopLevelSpace(parts[0])
		// First von Last
		vonStart, vonEnd := -1, -1
		for i, t := range toks {
			if i == len(toks)-1 {
				break // last token is always Last
			}
			if isVonToken(t) {
				if vonStart < 0 {
					vonStart = i
				}
				vonEnd = i
			}
		}
		if vonStart < 0 {
			if len(toks) > 0 {
				n.Last = toks[len(toks)-1]
				n.First = strings.Join(toks[:len(toks)-1], " ")
			}
		} else {
			n.First = strings.Join(toks[:vonStart], " ")
			n.von = strings.Join(toks[vonStart:vonEnd+1], " ")
			n.Last = strings.Join(toks[vonEnd+1:], " ")
		}
	case 2:
		n.von, n.Last = splitVonLast(parts[0])
		n.First = parts[1]
	default:
		n.von, n.Last = splitVonLast(parts[0])
		n.Jr = parts[1]
		n.First = strings.Join(parts[2:], " ")
	}
	return n
}

func splitVonLast(s string) (von, last string) {
	toks := splitTopLevelSpace(s)
	vonEnd := -1
	for i, t := range toks {
		if i == len(toks)-1 {
			break
		}
		if isVonToken(t) {
			vonEnd = i
		} else if vonEnd >= 0 {
			break
		}
	}
	if vonEnd < 0 {
		return "", strings.Join(toks, " ")
	}
	return strings.Join(toks[:vonEnd+1], " "), strings.Join(toks[vonEnd+1:], " ")
}

// isVonToken reports whether a token's first cased letter is lowercase.
// Braced tokens ({\v S}iki\'{c}) are treated as uppercase-protected, matching
// BibTeX, and TeX accent macros are looked through so \'{e}tienne counts as
// lowercase.
func isVonToken(tok string) bool {
	if tok == "" {
		return false
	}
	if strings.HasPrefix(tok, "{") {
		return false
	}
	plain, _ := Detex(tok)
	for _, r := range plain {
		if unicode.IsUpper(r) {
			return false
		}
		if unicode.IsLower(r) {
			return true
		}
	}
	return false
}

func matchedOuter(s string) bool {
	depth := 0
	for i, r := range s {
		if r == '{' {
			depth++
		} else if r == '}' {
			depth--
			if depth == 0 && i != len(s)-1 {
				return false
			}
		}
	}
	return depth == 0
}

func splitTopLevel(s string, sep rune) []string {
	var out []string
	depth := 0
	start := 0
	r := []rune(s)
	for i := 0; i < len(r); i++ {
		switch r[i] {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case '\\':
			i++
		case sep:
			if depth == 0 {
				out = append(out, string(r[start:i]))
				start = i + 1
			}
		}
	}
	out = append(out, string(r[start:]))
	return out
}

func splitTopLevelSpace(s string) []string {
	var out []string
	depth := 0
	var cur strings.Builder
	r := []rune(s)
	for i := 0; i < len(r); i++ {
		c := r[i]
		switch {
		case c == '{':
			depth++
			cur.WriteRune(c)
		case c == '}':
			if depth > 0 {
				depth--
			}
			cur.WriteRune(c)
		case c == '\\' && i+1 < len(r):
			cur.WriteRune(c)
			i++
			cur.WriteRune(r[i])
		case isSpace(c) && depth == 0:
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(c)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// FamilyASCII returns the family name reduced to ASCII letters, suitable for a
// cite key. The pipeline is: detex -> fold -> slug, in that order. Doing it in
// any other order is what produces keys like Mari__2024: a slug pass that runs
// before the TeX decode sees \'{c} as punctuation and deletes it.
func (n Name) FamilyASCII(includeVon bool) (string, []rune, []string) {
	raw := n.Last
	if includeVon && n.von != "" {
		raw = n.von + " " + n.Last
	}
	detexed, warns := Detex(raw)
	slug, residue := Slug(detexed, false)
	return slug, residue, warns
}

// String renders the name back in "von Last, Jr, First" form.
func (n Name) String() string {
	if n.Literal {
		return "{" + n.Last + "}"
	}
	last := strings.TrimSpace(n.von + " " + n.Last)
	switch {
	case n.Jr != "" && n.First != "":
		return last + ", " + n.Jr + ", " + n.First
	case n.Jr != "":
		return last + ", " + n.Jr
	case n.First != "":
		return last + ", " + n.First
	default:
		return last
	}
}

// Detexed returns a copy of the name with all TeX markup resolved to Unicode.
func (n Name) Detexed() Name {
	d := n
	d.First, _ = Detex(n.First)
	d.von, _ = Detex(n.von)
	d.Last, _ = Detex(n.Last)
	d.Jr, _ = Detex(n.Jr)
	return d
}

// FoldedASCII returns a copy of the name folded all the way to ASCII.
func (n Name) FoldedASCII() Name {
	d := n.Detexed()
	d.First, _ = Fold(d.First)
	d.von, _ = Fold(d.von)
	d.Last, _ = Fold(d.Last)
	d.Jr, _ = Fold(d.Jr)
	return d
}
