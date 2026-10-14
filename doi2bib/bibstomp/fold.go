package bibstomp

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// atomic holds the Latin letters that have NO canonical decomposition. NFD
// leaves these completely untouched, which is why the naive
// "decompose and drop combining marks" recipe silently fails on Polish,
// Nordic, Icelandic, Croatian, Vietnamese and Maltese names.
var atomic = map[rune]string{
	'ł': "l", 'Ł': "L",
	'ø': "o", 'Ø': "O",
	'đ': "d", 'Đ': "D",
	'ð': "d", 'Ð': "D",
	'þ': "th", 'Þ': "Th",
	'ħ': "h", 'Ħ': "H",
	'ŧ': "t", 'Ŧ': "T",
	'ı': "i", 'İ': "I",
	'ȷ': "j",
	'ŋ': "ng", 'Ŋ': "Ng",
	'æ': "ae", 'Æ': "Ae",
	'œ': "oe", 'Œ': "Oe",
	'ß': "ss", 'ẞ': "SS",
	'ĸ': "k", 'ſ': "s",
	'ƀ': "b", 'Ƀ': "B",
	'ə': "e", 'Ə': "E",
	'ɛ': "e", 'Ɛ': "E",
	'ɔ': "o", 'Ɔ': "O",
	'ʉ': "u", 'Ʉ': "U",
	'ɨ': "i", 'Ɨ': "I",
	'ǝ': "e",
	'ƶ': "z", 'Ƶ': "Z",
	'ȩ': "e", 'Ȩ': "E",
	'ƴ': "y", 'Ƴ': "Y",
	'ƙ': "k", 'Ƙ': "K",
	'ƭ': "t", 'ƪ': "l",
	'ǥ': "g", 'Ǥ': "G",
	'ȝ': "g", 'Ȝ': "G",
	'ƣ': "oi", 'Ƣ': "Oi",
	'ĳ': "ij", 'Ĳ': "IJ",
	'ŉ': "n",
	'ǆ': "dz", 'ǅ': "Dz", 'Ǆ': "DZ",
	'ǉ': "lj", 'ǈ': "Lj", 'Ǉ': "LJ",
	'ǌ': "nj", 'ǋ': "Nj", 'Ǌ': "NJ",
	// ligatures that show up in OCR-sourced bib files
	'ﬀ': "ff", 'ﬁ': "fi", 'ﬂ': "fl", 'ﬃ': "ffi", 'ﬄ': "ffl", 'ﬅ': "st",
	// punctuation that is technically non-ASCII and technically a name part
	'\u2018': "'", '\u2019': "'", '\u201a': "'", '\u201b': "'",
	'\u201c': "\"", '\u201d': "\"", '\u201e': "\"",
	'\u2010': "-", '\u2011': "-", '\u2012': "-", '\u2013': "-",
	'\u2014': "-", '\u2015': "-", '\u2212': "-",
	'\u00a0': " ", '\u2007': " ", '\u202f': " ", '\u2009': " ",
	'\u00ad': "", // soft hyphen
	'\u2026': "...",
	'\u00b7': ".", '\u2022': ".",
}

// Fold reduces s to ASCII in two passes, because there are two disjoint
// classes of "strange letter" and neither pass alone covers both.
//
//	pass 1: NFD + drop combining marks   -> handles á é ñ ü ç ō š ć
//	pass 2: explicit atomic rune table   -> handles ł ø đ þ æ ß ŋ ı
//
// Anything still non-ASCII after both passes (Cyrillic, Han, Arabic, Greek)
// is returned in residue rather than silently dropped, so the caller can flag
// the entry for a human instead of minting a mangled key.
func Fold(s string) (folded string, residue []rune) {
	// Transformers carry mutable buffers; each library call owns its chain.
	stripMarks := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	stripped, _, err := transform.String(stripMarks, s)
	if err != nil {
		stripped = s
	}
	var b strings.Builder
	b.Grow(len(stripped))
	seen := map[rune]bool{}
	for _, r := range stripped {
		switch {
		case r < unicode.MaxASCII:
			b.WriteRune(r)
		default:
			if rep, ok := atomic[r]; ok {
				b.WriteString(rep)
				continue
			}
			if !seen[r] {
				seen[r] = true
				residue = append(residue, r)
			}
		}
	}
	return b.String(), residue
}

// Slug folds s and then reduces it to characters legal in a BibTeX cite key.
// BibTeX itself forbids the separators ",{}()= and whitespace; biber/biblatex
// is stricter in practice, so we keep to [A-Za-z0-9] plus an optional
// allowance for hyphens and colons.
func Slug(s string, keepHyphen bool) (string, []rune) {
	folded, residue := Fold(s)
	var b strings.Builder
	for _, r := range folded {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case keepHyphen && r == '-':
			b.WriteRune('-')
		}
	}
	return b.String(), residue
}
