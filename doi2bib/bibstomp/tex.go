package bibstomp

import (
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// accentMacros maps a TeX accent command to the Unicode combining mark it
// represents. Using combining marks rather than a (accent x base) lookup table
// means we get full coverage for free: \v{c} and \v{Z} and \v{g} all work
// without enumerating them, and NFC composes the ones that have precomposed
// forms.
var accentMacros = map[string]rune{
	"'":  0x0301, // acute        \'{c} -> ć
	"`":  0x0300, // grave        \`{a} -> à
	"^":  0x0302, // circumflex   \^{o} -> ô
	"\"": 0x0308, // diaeresis    \"{u} -> ü
	"~":  0x0303, // tilde        \~{n} -> ñ
	"=":  0x0304, // macron       \={o} -> ō
	".":  0x0307, // dot above    \.{z} -> ż
	"u":  0x0306, // breve        \u{g} -> ğ
	"v":  0x030C, // caron/hacek  \v{s} -> š
	"H":  0x030B, // double acute \H{o} -> ő
	"r":  0x030A, // ring above   \r{a} -> å
	"c":  0x0327, // cedilla      \c{c} -> ç
	"k":  0x0328, // ogonek       \k{a} -> ą
	"d":  0x0323, // dot below    \d{s} -> ṣ
	"b":  0x0331, // macron below \b{h} -> ẖ
	"G":  0x030F, // double grave (yannis) \G{o}
	"C":  0x030F, // double grave (alt)
	"f":  0x0311, // inverted breve
	"U":  0x030B, // double acute (alt)
}

// symbolMacros maps standalone TeX letter/symbol commands to their Unicode
// equivalent. These are the "atomic" letters -- the ones with no canonical
// decomposition, which is exactly why they need an explicit table both here
// and again in fold.go.
var symbolMacros = map[string]string{
	"l": "ł", "L": "Ł",
	"o": "ø", "O": "Ø",
	"i": "ı", "j": "ȷ",
	"ss": "ß", "SS": "SS",
	"ae": "æ", "AE": "Æ",
	"oe": "œ", "OE": "Œ",
	"aa": "å", "AA": "Å",
	"dh": "ð", "DH": "Ð",
	"th": "þ", "TH": "Þ",
	"dj": "đ", "DJ": "Đ",
	"ng": "ŋ", "NG": "Ŋ",
	"textquotesingle": "'",
	"textquoteleft":   "\u2018",
	"textquoteright":  "\u2019",
	"textendash":      "\u2013",
	"textemdash":      "\u2014",
	"textbullet":      "\u2022",
	"textdegree":      "\u00b0",
	"textprime":       "\u2032",
	"guillemotleft":   "\u00ab",
	"guillemotright":  "\u00bb",
	"pounds":          "£",
	"copyright":       "©",
	"textcopyright":   "©",
	"P":               "¶",
	"S":               "§",
	// explicit spacing control symbols
	" ": " ", "\t": " ", "\n": " ",
	",": "\u2009", ";": " ", ":": " ", "!": "", "/": "",
	// escaped literals
	"&": "&", "%": "%", "$": "$", "#": "#", "_": "_",
	"{": "{", "}": "}",
	"relax": "",
}

// passthroughMacros take one argument that should be emitted verbatim (the
// command itself contributes nothing). Kept separate from symbolMacros so the
// scanner knows to consume the following group.
var passthroughMacros = map[string]bool{
	"textit": true, "textbf": true, "textrm": true, "textsc": true,
	"emph": true, "mbox": true, "text": true, "textnormal": true,
	"protect": true, "noopsort": true,
}

type detexer struct {
	src   []rune
	pos   int
	out   strings.Builder
	warns map[string]int
}

// Detex converts TeX markup in s into plain Unicode. It reports any macros it
// did not recognise, so callers can surface them rather than silently
// mangling a name.
func Detex(s string) (string, []string) {
	if !strings.ContainsAny(s, `\{}~`) {
		return s, nil
	}
	d := &detexer{src: []rune(s), warns: map[string]int{}}
	d.run()
	out := norm.NFC.String(d.out.String())

	var unknown []string
	for m := range d.warns {
		unknown = append(unknown, m)
	}
	sort.Strings(unknown)
	return out, unknown
}

func (d *detexer) run() {
	for d.pos < len(d.src) {
		c := d.src[d.pos]
		switch c {
		case '\\':
			d.pos++
			d.macro()
		case '{', '}':
			// Grouping braces carry no textual content. Brace-protection of
			// capitalisation is handled upstream by the name parser, which
			// runs before Detex.
			d.pos++
		case '~':
			d.out.WriteRune(' ') // non-breaking space
			d.pos++
		default:
			d.out.WriteRune(c)
			d.pos++
		}
	}
}

func (d *detexer) macro() {
	if d.pos >= len(d.src) {
		return
	}
	name := d.readMacroName()
	isWord := len(name) > 0 && unicode.IsLetter(rune(name[0]))

	if passthroughMacros[name] {
		if isWord {
			d.skipSpaces()
		}
		arg, _ := d.readArg(isWord)
		d.out.WriteString(arg)
		return
	}
	if rep, ok := symbolMacros[name]; ok {
		d.out.WriteString(rep)
		if isWord {
			// \l{} and \l  differ: an explicit empty group terminates the
			// control word, so any space after it is a real space. Only a
			// bare control word eats the space that delimits it.
			if !d.consumeEmptyGroup() {
				d.skipSpaces()
			}
		}
		return
	}
	if mark, ok := accentMacros[name]; ok {
		arg, ok := d.readArg(isWord)
		if !ok {
			// dangling accent with no base; drop it rather than emit a
			// floating combining mark
			return
		}
		r := []rune(arg)
		if len(r) == 0 {
			return
		}
		// TeX writes \'{\i} because the accent replaces the dot; Unicode
		// wants the dotted letter plus a combining mark, so undo the
		// dotlessness before composing.
		switch r[0] {
		case 'ı':
			r[0] = 'i'
		case 'ȷ':
			r[0] = 'j'
		}
		// Attach the mark to the first rune of the argument; NFC will compose
		// it if a precomposed form exists.
		d.out.WriteRune(r[0])
		d.out.WriteRune(mark)
		d.out.WriteString(string(r[1:]))
		return
	}

	// Unknown macro. Drop the command, keep any argument, and record it.
	d.warns["\\"+name]++
	if isWord {
		d.skipSpaces()
	}
	if d.pos < len(d.src) && d.src[d.pos] == '{' {
		arg, _ := d.readArg(isWord)
		d.out.WriteString(arg)
	}
}

// readMacroName reads a control word (all following letters) or a control
// symbol (exactly one non-letter character).
func (d *detexer) readMacroName() string {
	if d.pos >= len(d.src) {
		return ""
	}
	if !unicode.IsLetter(d.src[d.pos]) {
		n := string(d.src[d.pos])
		d.pos++
		return n
	}
	start := d.pos
	for d.pos < len(d.src) && unicode.IsLetter(d.src[d.pos]) {
		d.pos++
	}
	name := string(d.src[start:d.pos])
	// Greedy letter-reading would swallow \ss out of \sspam, but it also
	// mis-splits \vs. Prefer the longest known symbol macro prefix.
	if _, ok := symbolMacros[name]; ok {
		return name
	}
	if _, ok := accentMacros[name]; ok {
		return name
	}
	for n := len(name) - 1; n >= 1; n-- {
		pre := name[:n]
		_, a := accentMacros[pre]
		_, s := symbolMacros[pre]
		if a || s {
			d.pos -= len(name) - n
			return pre
		}
	}
	return name
}

func (d *detexer) skipSpaces() {
	for d.pos < len(d.src) && (d.src[d.pos] == ' ' || d.src[d.pos] == '\t' || d.src[d.pos] == '\n') {
		d.pos++
	}
}

func (d *detexer) consumeEmptyGroup() bool {
	if d.pos+1 < len(d.src) && d.src[d.pos] == '{' && d.src[d.pos+1] == '}' {
		d.pos += 2
		return true
	}
	return false
}

// readArg reads a macro argument: either a braced group (recursively detexed),
// a nested macro such as \i, or a single character.
func (d *detexer) readArg(skipSpace bool) (string, bool) {
	if skipSpace {
		d.skipSpaces()
	}
	if d.pos >= len(d.src) {
		return "", false
	}
	switch d.src[d.pos] {
	case '{':
		depth := 0
		start := d.pos + 1
		for d.pos < len(d.src) {
			if d.src[d.pos] == '{' {
				depth++
			} else if d.src[d.pos] == '}' {
				depth--
				if depth == 0 {
					inner := string(d.src[start:d.pos])
					d.pos++
					got, w := Detex(inner)
					for _, m := range w {
						d.warns[m]++
					}
					return got, true
				}
			}
			d.pos++
		}
		inner := string(d.src[start:])
		d.pos = len(d.src)
		got, _ := Detex(inner)
		return got, true
	case '\\':
		d.pos++
		sub := &detexer{src: d.src, pos: d.pos, warns: d.warns}
		sub.macro()
		d.pos = sub.pos
		return sub.out.String(), true
	default:
		r := d.src[d.pos]
		d.pos++
		return string(r), true
	}
}
