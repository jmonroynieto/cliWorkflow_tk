package bibstomp

import (
	"fmt"
	"strings"
)

// Field is one name = value pair, with byte offsets into the source so the
// value can be spliced without re-serialising the whole file.
type Field struct {
	Name       string // lowercased
	RawName    string
	Value      string // brace/quote delimiters stripped, macros left alone
	ValueStart int    // offset of first byte inside the delimiter
	ValueEnd   int    // offset just past the last byte inside the delimiter
	Braced     bool
}

// Entry is one @type{...} record.
type Entry struct {
	Type     string // lowercased, e.g. "article"
	Key      string
	KeyStart int
	KeyEnd   int
	Fields   []Field
	Start    int
	End      int
	Special  bool // @string, @preamble, @comment: preserved verbatim
}

func (e *Entry) Field(name string) (Field, bool) {
	for _, f := range e.Fields {
		if f.Name == name {
			return f, true
		}
	}
	return Field{}, false
}

func (e *Entry) FieldValue(name string) string {
	f, ok := e.Field(name)
	if !ok {
		return ""
	}
	return f.Value
}

type parser struct {
	src []byte
	pos int
}

// ParseBib scans a .bib file into entries. It is deliberately forgiving: an
// entry it cannot make sense of is skipped with a warning rather than aborting
// the run, because real-world .bib files accumulate damage.
func ParseBib(src []byte) ([]*Entry, []string) {
	p := &parser{src: src}
	var entries []*Entry
	var warns []string
	for {
		if !p.seekAt() {
			break
		}
		start := p.pos
		e, err := p.entry()
		if err != nil {
			warns = append(warns, fmt.Sprintf("offset %d: %v", start, err))
			p.pos = start + 1
			continue
		}
		entries = append(entries, e)
	}
	return entries, warns
}

func (p *parser) seekAt() bool {
	for p.pos < len(p.src) {
		if p.src[p.pos] == '@' {
			return true
		}
		p.pos++
	}
	return false
}

func (p *parser) entry() (*Entry, error) {
	e := &Entry{Start: p.pos}
	p.pos++ // consume '@'
	p.ws()
	tstart := p.pos
	for p.pos < len(p.src) && isIdentByte(p.src[p.pos]) {
		p.pos++
	}
	e.Type = strings.ToLower(string(p.src[tstart:p.pos]))
	if e.Type == "" {
		return nil, fmt.Errorf("empty entry type")
	}
	p.ws()
	if p.pos >= len(p.src) {
		return nil, fmt.Errorf("truncated entry")
	}
	open := p.src[p.pos]
	var close byte
	switch open {
	case '{':
		close = '}'
	case '(':
		close = ')'
	default:
		return nil, fmt.Errorf("expected { after @%s", e.Type)
	}
	p.pos++

	if e.Type == "string" || e.Type == "preamble" || e.Type == "comment" {
		e.Special = true
		p.skipBalanced(open, close)
		e.End = p.pos
		return e, nil
	}

	p.ws()
	e.KeyStart = p.pos
	for p.pos < len(p.src) && p.src[p.pos] != ',' && p.src[p.pos] != close {
		p.pos++
	}
	e.KeyEnd = p.pos
	e.Key = strings.TrimSpace(string(p.src[e.KeyStart:e.KeyEnd]))
	// tighten offsets onto the key text itself
	for e.KeyStart < e.KeyEnd && isSpaceByte(p.src[e.KeyStart]) {
		e.KeyStart++
	}
	for e.KeyEnd > e.KeyStart && isSpaceByte(p.src[e.KeyEnd-1]) {
		e.KeyEnd--
	}

	for p.pos < len(p.src) && p.src[p.pos] == ',' {
		p.pos++
		p.ws()
		if p.pos < len(p.src) && p.src[p.pos] == close {
			break
		}
		f, ok := p.field()
		if !ok {
			break
		}
		e.Fields = append(e.Fields, f)
		p.ws()
	}
	if p.pos < len(p.src) && p.src[p.pos] == close {
		p.pos++
	}
	e.End = p.pos
	return e, nil
}

func (p *parser) field() (Field, bool) {
	p.ws()
	ns := p.pos
	for p.pos < len(p.src) && isIdentByte(p.src[p.pos]) {
		p.pos++
	}
	if p.pos == ns {
		return Field{}, false
	}
	f := Field{RawName: string(p.src[ns:p.pos])}
	f.Name = strings.ToLower(f.RawName)
	p.ws()
	if p.pos >= len(p.src) || p.src[p.pos] != '=' {
		return f, false
	}
	p.pos++
	p.ws()
	if p.pos >= len(p.src) {
		return f, false
	}
	switch p.src[p.pos] {
	case '{':
		f.Braced = true
		p.pos++
		f.ValueStart = p.pos
		p.skipBalanced('{', '}')
		f.ValueEnd = p.pos - 1
	case '"':
		p.pos++
		f.ValueStart = p.pos
		depth := 0
		for p.pos < len(p.src) {
			c := p.src[p.pos]
			if c == '\\' {
				p.pos += 2
				continue
			}
			if c == '{' {
				depth++
			} else if c == '}' && depth > 0 {
				depth--
			} else if c == '"' && depth == 0 {
				break
			}
			p.pos++
		}
		f.ValueEnd = p.pos
		p.pos++
	default:
		f.ValueStart = p.pos
		for p.pos < len(p.src) && p.src[p.pos] != ',' && p.src[p.pos] != '}' && p.src[p.pos] != ')' && p.src[p.pos] != '\n' {
			p.pos++
		}
		f.ValueEnd = p.pos
	}
	if f.ValueEnd < f.ValueStart {
		f.ValueEnd = f.ValueStart
	}
	f.Value = strings.TrimSpace(string(p.src[f.ValueStart:f.ValueEnd]))
	return f, true
}

// skipBalanced assumes pos is just past the opening delimiter and advances to
// just past its match.
func (p *parser) skipBalanced(open, close byte) {
	depth := 1
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if c == '\\' {
			p.pos += 2
			continue
		}
		if c == open {
			depth++
		} else if c == close {
			depth--
			if depth == 0 {
				p.pos++
				return
			}
		}
		p.pos++
	}
}

func (p *parser) ws() {
	for p.pos < len(p.src) && isSpaceByte(p.src[p.pos]) {
		p.pos++
	}
}

func isSpaceByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func isIdentByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
		c == '-' || c == '_' || c == '.' || c == '+' || c == ':' || c == '/'
}

// Splice is a byte-range replacement to be applied to the source.
type Splice struct {
	Start, End int
	Text       string
}

// ApplySplices rewrites src with the given replacements. Splices must not
// overlap; they are sorted internally so callers can emit them in any order.
func ApplySplices(src []byte, sp []Splice) []byte {
	for i := 1; i < len(sp); i++ {
		for j := i; j > 0 && sp[j].Start < sp[j-1].Start; j-- {
			sp[j], sp[j-1] = sp[j-1], sp[j]
		}
	}
	var b strings.Builder
	prev := 0
	for _, s := range sp {
		if s.Start < prev {
			continue // overlapping; skip defensively
		}
		b.Write(src[prev:s.Start])
		b.WriteString(s.Text)
		prev = s.End
	}
	b.Write(src[prev:])
	return []byte(b.String())
}
