package airr

// Bounded AIRR YAML subset, written from YAML 1.2.2 productions, not library source:
// https://yaml.org/spec/1.2.2/ (block collections, scalars and core resolution).
// See docs/AIRR-YAML.md for accepted syntax and deliberate refusals. Values go
// directly to exact JSON; neither numeric resolution nor evidence uses float64.
import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const maxYAMLLines = 1_000_000
const maxYAMLLineBytes = 1 << 20

var yamlJSONNumber = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)
var yamlOtherNumber = regexp.MustCompile(`^[+-]?(?:[0-9]|\.[0-9])`)

type yamlLine struct {
	text           string
	number, indent int
}
type yamlReader struct {
	ctx        context.Context
	lines      []yamlLine
	at, values int
}

// ReadRepertoireYAML decodes the documented subset and reuses the JSON import
// checks. Raw on its records is exact decoded JSON, NOT original YAML source.
func ReadRepertoireYAML(ctx context.Context, data []byte) (RepertoireDocument, error) {
	encoded, err := decodeMetadataYAML(ctx, data)
	if err != nil {
		return RepertoireDocument{}, err
	}
	return ReadRepertoireDocument(ctx, encoded)
}

func decodeMetadataYAML(ctx context.Context, data []byte) ([]byte, error) {
	var empty []byte
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	if len(data) > maxMetadataBytes {
		return empty, fmt.Errorf("YAML exceeds 64 MiB")
	}
	if !utf8.Valid(data) {
		return empty, fmt.Errorf("YAML must be valid UTF-8")
	}
	// JSON is also a supported YAML serialization; retain its original object bytes.
	if json.Valid(data) {
		if err := yamlSurrogates(string(data)); err != nil {
			return empty, err
		}
		return data, nil
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if strings.ContainsAny(text, "\r\t") {
		return empty, fmt.Errorf("YAML: bare CR and literal tabs are unsupported; use LF/CRLF and spaces")
	}
	for _, r := range text {
		if (r < 0x20 && r != '\n') || r == 0x7f || (r >= 0x80 && r <= 0x9f) || r == 0xfeff || r == 0xfffe || r == 0xffff {
			return empty, fmt.Errorf("YAML contains an unsupported control/BOM character U+%04X", r)
		}
	}
	if strings.Count(text, "\n") >= maxYAMLLines {
		return empty, fmt.Errorf("YAML exceeds %d lines", maxYAMLLines)
	}
	p := yamlReader{ctx: ctx}
	for i, s := range strings.Split(text, "\n") {
		if i%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return empty, err
			}
		}
		if len(s) > maxYAMLLineBytes {
			return empty, fmt.Errorf("YAML line %d exceeds 1 MiB", i+1)
		}
		p.lines = append(p.lines, yamlLine{s, i + 1, len(s) - len(strings.TrimLeft(s, " "))})
	}
	p.skip()
	if p.at < len(p.lines) && p.clean(p.lines[p.at]) == "---" {
		p.at++
		p.skip()
	}
	if p.at == len(p.lines) {
		return empty, fmt.Errorf("YAML contains no document")
	}
	if p.lines[p.at].indent != 0 {
		return empty, p.err(p.lines[p.at], "the root must start in column 1")
	}
	value, err := p.node(0, 0)
	if err != nil {
		return empty, err
	}
	p.skip()
	if p.at < len(p.lines) && p.clean(p.lines[p.at]) == "..." && p.lines[p.at].indent == 0 {
		p.at++
		p.skip()
	}
	if p.at != len(p.lines) {
		return empty, p.err(p.lines[p.at], "unexpected content or multiple documents")
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return empty, err
	}
	if len(encoded) > maxMetadataBytes {
		return empty, fmt.Errorf("decoded YAML JSON exceeds 64 MiB")
	}
	return encoded, nil
}
func (p *yamlReader) err(l yamlLine, message string) error {
	return fmt.Errorf("YAML line %d, column %d: %s (near %.120q); see docs/AIRR-YAML.md or export JSON", l.number, l.indent+1, message, l.text[l.indent:])
}
func (p *yamlReader) step(l yamlLine, depth int) error {
	if err := p.ctx.Err(); err != nil {
		return err
	}
	p.values++
	if depth > maxMetadataDepth || p.values > maxMetadataValues {
		return p.err(l, "nesting/value budget exceeded")
	}
	return nil
}

// Comments are stripped only outside quotes and after whitespace. A quote is a
// delimiter only at the beginning of a token; apostrophes in plain prose remain text.
func (p *yamlReader) clean(l yamlLine) string {
	s := l.text[l.indent:]
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == '\\' && quote == '"' {
				i++
				continue
			}
			if c == quote {
				if quote == '\'' && i+1 < len(s) && s[i+1] == '\'' {
					i++
					continue
				}
				quote = 0
			}
			continue
		}
		if (c == '"' || c == '\'') && (i == 0 || strings.ContainsRune(" [{,:-", rune(s[i-1]))) {
			quote = c
		}
		if c == '#' && (i == 0 || s[i-1] == ' ') {
			return strings.Trim(s[:i], " ")
		}
	}
	return strings.Trim(s, " ")
}
func (p *yamlReader) skip() {
	for p.at < len(p.lines) && p.clean(p.lines[p.at]) == "" {
		p.at++
	}
}
func yamlDash(s string) bool { return s == "-" || strings.HasPrefix(s, "- ") }

// Locate a block mapping delimiter, respecting quoted keys and flow collections.
func yamlColon(s string) int {
	var quote byte
	level := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == '\\' && quote == '"' {
				i++
				continue
			}
			if c == quote {
				if quote == '\'' && i+1 < len(s) && s[i+1] == '\'' {
					i++
					continue
				}
				quote = 0
			}
			continue
		}
		if (c == '\'' || c == '"') && (i == 0 || strings.ContainsRune(" [{,:", rune(s[i-1]))) {
			quote = c
			continue
		}
		if c == '[' || c == '{' {
			level++
		}
		if c == ']' || c == '}' {
			level--
		}
		if c == ':' && level == 0 && (i+1 == len(s) || s[i+1] == ' ') {
			return i
		}
	}
	return -1
}
func (p *yamlReader) node(indent, depth int) (any, error) {
	p.skip()
	if p.at == len(p.lines) {
		return nil, nil
	}
	l := p.lines[p.at]
	if err := p.step(l, depth); err != nil {
		return nil, err
	}
	if l.indent != indent {
		return nil, p.err(l, "unexpected indentation")
	}
	s := p.clean(l)
	if yamlDash(s) {
		out := []any{}
		for p.at < len(p.lines) {
			p.skip()
			if p.at == len(p.lines) {
				break
			}
			l = p.lines[p.at]
			s = p.clean(l)
			if l.indent != indent || !yamlDash(s) {
				break
			}
			p.at++
			rest := strings.Trim(s[1:], " ")
			var v any
			var err error
			if rest == "" {
				v, err = p.child(indent, depth+1, false)
			} else if yamlColon(rest) >= 0 {
				// The compact mapping's keys align after '- ', not with the dash.
				spaces := len(s[1:]) - len(strings.TrimLeft(s[1:], " "))
				virtual := yamlLine{strings.Repeat(" ", indent+1+spaces) + rest, l.number, indent + 1 + spaces}
				v, err = p.mapping(virtual, depth+1)
			} else {
				v, err = p.scalar(rest, l, depth+1)
			}
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	}
	if yamlColon(s) >= 0 {
		p.at++
		return p.mapping(l, depth)
	}
	p.at++
	return p.scalar(s, l, depth)
}
func (p *yamlReader) child(parent, depth int, indentless bool) (any, error) {
	p.skip()
	if p.at == len(p.lines) {
		return nil, nil
	}
	l := p.lines[p.at]
	if l.indent > parent || (indentless && l.indent == parent && yamlDash(p.clean(l))) {
		return p.node(l.indent, depth)
	}
	return nil, nil
}
func (p *yamlReader) mapping(first yamlLine, depth int) (any, error) {
	out := map[string]any{}
	l := first
	for {
		if err := p.step(l, depth); err != nil {
			return nil, err
		}
		s := p.clean(l)
		colon := yamlColon(s)
		if colon < 0 {
			return nil, p.err(l, "expected a mapping key followed by ': '")
		}
		keyText := strings.Trim(s[:colon], " ")
		if strings.HasPrefix(keyText, "|") || strings.HasPrefix(keyText, ">") {
			return nil, p.err(l, "block scalars cannot be mapping keys")
		}
		keyValue, err := p.scalar(keyText, l, depth+1)
		if err != nil {
			return nil, err
		}
		key, ok := keyValue.(string)
		if !ok || key == "" || key == "<<" {
			return nil, p.err(l, "keys must be nonempty strings; merge keys are unsupported")
		}
		if _, has := out[key]; has {
			return nil, p.err(l, "duplicate key "+strconv.Quote(key))
		}
		rest := strings.Trim(s[colon+1:], " ")
		var value any
		if rest == "" {
			value, err = p.child(l.indent, depth+1, true)
		} else {
			value, err = p.scalar(rest, l, depth+1)
		}
		if err != nil {
			return nil, err
		}
		out[key] = value
		p.skip()
		if p.at == len(p.lines) || p.lines[p.at].indent != first.indent || yamlDash(p.clean(p.lines[p.at])) {
			break
		}
		l = p.lines[p.at]
		if p.clean(l) == "..." || p.clean(l) == "---" {
			break
		}
		p.at++
	}
	return out, nil
}
func (p *yamlReader) scalar(s string, l yamlLine, depth int) (any, error) {
	if err := p.step(l, depth); err != nil {
		return nil, err
	}
	if s == "|" || s == "|-" || s == "|+" {
		return p.literal(s, l)
	}
	if s == "" {
		return nil, nil
	}
	if s[0] == '[' || s[0] == '{' {
		if err := yamlSurrogates(s); err != nil {
			return nil, p.err(l, err.Error())
		}
		if err := checkMetadataJSON(p.ctx, []byte(s)); err != nil {
			return nil, p.err(l, "inline collections must be strict JSON without duplicate keys: "+err.Error())
		}
		var v any
		d := json.NewDecoder(strings.NewReader(s))
		d.UseNumber()
		if err := d.Decode(&v); err != nil {
			return nil, p.err(l, err.Error())
		}
		if err := p.countFlow(v, l, depth); err != nil {
			return nil, err
		}
		return v, nil
	}
	if s[0] == '"' {
		if err := yamlSurrogates(s); err != nil {
			return nil, p.err(l, err.Error())
		}
		if err := checkMetadataJSON(p.ctx, []byte(s)); err != nil {
			return nil, p.err(l, "double-quoted scalars must use single-line JSON string syntax")
		}
		var v string
		if err := json.Unmarshal([]byte(s), &v); err != nil {
			return nil, p.err(l, "expected a quoted string")
		}
		return v, nil
	}
	if s[0] == '\'' {
		var b strings.Builder
		for i := 1; i < len(s); i++ {
			if s[i] == '\'' {
				if i+1 < len(s) && s[i+1] == '\'' {
					b.WriteByte('\'')
					i++
					continue
				}
				if i != len(s)-1 {
					return nil, p.err(l, "content after closing quote")
				}
				return b.String(), nil
			}
			b.WriteByte(s[i])
		}
		return nil, p.err(l, "unterminated single-quoted scalar; multiline quoted scalars are unsupported")
	}
	if strings.ContainsAny(s, "\"'") || strings.Contains(s, ": ") || strings.HasSuffix(s, ":") || strings.ContainsRune("!&*|>@`%[]{},", rune(s[0])) || s == "---" || s == "..." || s == "?" || strings.HasPrefix(s, "? ") || yamlDash(s) {
		return nil, p.err(l, "unsupported YAML syntax; quote a literal string")
	}
	switch s {
	case "null", "Null", "NULL", "~":
		return nil, nil
	case "true", "True", "TRUE":
		return true, nil
	case "false", "False", "FALSE":
		return false, nil
	}
	if yamlJSONNumber.MatchString(s) {
		return json.Number(s), nil
	}
	// A conservative refusal prevents YAML 1.1/1.2 numeric differences from turning
	// an intended numeric value into text. Quote dates, digit-leading IDs and other
	// ambiguous strings; yes/no/on/off remain YAML 1.2 strings.
	if yamlOtherNumber.MatchString(s) || strings.EqualFold(s, ".inf") || strings.EqualFold(s, "+.inf") || strings.EqualFold(s, "-.inf") || strings.EqualFold(s, ".nan") {
		return nil, p.err(l, "numeric-looking scalars must use JSON decimal syntax; quote text identifiers/dates")
	}
	return s, nil
}
func (p *yamlReader) literal(header string, l yamlLine) (any, error) {
	start := p.at
	indent := -1
	for p.at < len(p.lines) {
		line := p.lines[p.at]
		if line.text != "" && strings.Trim(line.text, " ") == "" {
			return nil, p.err(line, "space-only literal lines must be empty")
		}
		if strings.Trim(line.text, " ") != "" {
			if line.indent <= l.indent {
				break
			}
			if indent < 0 {
				indent = line.indent
			}
			if line.indent < indent {
				return nil, p.err(line, "literal text is less indented than its first content line")
			}
		}
		p.at++
	}
	var b strings.Builder
	for i := start; i < p.at; i++ {
		if err := p.ctx.Err(); err != nil {
			return nil, err
		}
		line := p.lines[i]
		if strings.Trim(line.text, " ") != "" {
			b.WriteString(line.text[indent:])
		}
		if i < len(p.lines)-1 {
			b.WriteByte('\n')
		}
	}
	s := b.String()
	switch header {
	case "|-":
		s = strings.TrimRight(s, "\n")
	case "|":
		ended := strings.HasSuffix(s, "\n")
		s = strings.TrimRight(s, "\n")
		if s != "" && ended {
			s += "\n"
		}
	}
	return s, nil
}

func yamlSurrogates(s string) error {
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' {
			continue
		}
		i++
		if i >= len(s) {
			break
		}
		if s[i] == 'u' && i+4 < len(s) {
			n, err := strconv.ParseUint(s[i+1:i+5], 16, 16)
			if err == nil && n >= 0xd800 && n <= 0xdfff {
				return fmt.Errorf("YAML surrogate escapes are unsupported; use UTF-8 characters")
			}
		}
	}
	return nil
}
func (p *yamlReader) countFlow(v any, l yamlLine, depth int) error {
	if err := p.step(l, depth); err != nil {
		return err
	}
	switch x := v.(type) {
	case []any:
		for _, child := range x {
			if err := p.countFlow(child, l, depth+1); err != nil {
				return err
			}
		}
	case map[string]any:
		for key, child := range x {
			if key == "<<" {
				return p.err(l, "merge keys are unsupported")
			}
			if err := p.countFlow(child, l, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}
