// Package reads holds ClonoLoom's readers of repertoire files: each reads a vendor's or tool's
// format into the same rearrangement table (internal/rep), recording what it could not read as
// problems with their file, line, column and value.
package reads

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Limits on untrusted input: a line longer than this is refused rather than held in memory
// (a full AIRR row with alignments is a few kilobytes).
const maxLine = 16 << 20

// ErrLineTooLong: a line exceeds maxLine.
var ErrLineTooLong = errors.New("a line is longer than 16 MB: this is not a table ClonoLoom reads")

// decompress answers a reader of r's content, gunzipped when it starts with the gzip magic bytes,
// and whether it was.
func decompress(r io.Reader) (io.Reader, bool, error) {
	br := bufio.NewReaderSize(r, 1<<16)
	magic, err := br.Peek(2)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, false, err
	}
	if len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b {
		gz, err := gzip.NewReader(br)
		if err != nil {
			return nil, true, fmt.Errorf("the file looks gzipped but cannot be read: %w", err)
		}
		gz.Multistream(true)
		return bufio.NewReaderSize(gz, 1<<16), true, nil
	}
	return br, false, nil
}

// lines reads a text table line by line: lines end in LF or CRLF, a UTF-8 byte-order mark before
// the first is dropped, and the line number counts from 1.
type lines struct {
	r    *bufio.Reader
	n    uint32
	line []byte
	err  error
	ctx  context.Context
}

func newLines(r io.Reader) *lines {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReaderSize(r, 1<<16)
	}
	return &lines{r: br}
}

// next reads the next line (without its end); false at the end of the input or on an error, or
// when the context (checked every 65,536 lines) is done.
func (l *lines) next() bool {
	if l.ctx != nil && l.n&0xffff == 0xffff {
		if err := l.ctx.Err(); err != nil {
			l.err = err
			return false
		}
	}
	l.line = l.line[:0]
	for {
		chunk, err := l.r.ReadSlice('\n')
		l.line = append(l.line, chunk...)
		if len(l.line) > maxLine {
			l.err = ErrLineTooLong
			return false
		}
		if err == nil {
			break
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if errors.Is(err, io.EOF) {
			if len(l.line) == 0 {
				return false
			}
			break
		}
		l.err = err
		return false
	}
	l.n++
	l.line = bytes.TrimSuffix(l.line, []byte("\n"))
	l.line = bytes.TrimSuffix(l.line, []byte("\r"))
	if l.n == 1 {
		l.line = bytes.TrimPrefix(l.line, []byte("\xef\xbb\xbf"))
	}
	return true
}

// splitTab splits a tab-separated line into fields (no quoting: AIRR TSV and the vendors' tab
// tables do not quote).
func splitTab(line []byte, fields []string) []string {
	fields = fields[:0]
	for {
		i := bytes.IndexByte(line, '\t')
		if i < 0 {
			return append(fields, unquote(line))
		}
		fields = append(fields, unquote(line[:i]))
		line = line[i+1:]
	}
}

// unquote reads a tab-separated field as Python's csv module does: one in double quotes (as R's
// write.table writes every field, the AIRR R package's example file among them) loses them, a
// doubled quote inside standing for one. The AIRR standard does not quote, but files written by R
// do.
func unquote(b []byte) string {
	if len(b) < 2 || b[0] != '"' || b[len(b)-1] != '"' {
		return string(b)
	}
	inner := b[1 : len(b)-1]
	if bytes.IndexByte(inner, '"') < 0 {
		return string(inner)
	}
	return strings.ReplaceAll(string(inner), `""`, `"`)
}

// splitCSV splits a comma-separated line (RFC 4180 quoting, as Cell Ranger writes it) into fields.
// A quoted field may not span lines in the tables ClonoLoom reads; ok is false when quotes are
// unbalanced.
func splitCSV(line []byte, fields []string) ([]string, bool) {
	return splitSep(line, ',', fields)
}

// splitSep splits a line separated by sep, with RFC 4180's quoting, into fields.
func splitSep(line []byte, sep byte, fields []string) ([]string, bool) {
	fields = fields[:0]
	var field strings.Builder
	for i := 0; ; {
		field.Reset()
		if i < len(line) && line[i] == '"' {
			i++
			for {
				if i >= len(line) {
					return fields, false
				}
				if line[i] == '"' {
					if i+1 < len(line) && line[i+1] == '"' {
						field.WriteByte('"')
						i += 2
						continue
					}
					i++
					break
				}
				field.WriteByte(line[i])
				i++
			}
			if i < len(line) && line[i] != sep {
				return fields, false
			}
		} else {
			j := bytes.IndexByte(line[i:], sep)
			if j < 0 {
				j = len(line) - i
			}
			field.Write(line[i : i+j])
			i += j
		}
		fields = append(fields, field.String())
		if i >= len(line) {
			return fields, true
		}
		i++ // the separator
		if i == len(line) {
			return append(fields, ""), true
		}
	}
}

// columns maps a header's names to their indexes; names are compared as given and, failing that,
// without regard to case.
type columns map[string]int

func newColumns(header []string) columns {
	c := columns{}
	for i, name := range header {
		name = strings.TrimSpace(name)
		if _, dup := c[name]; !dup {
			c[name] = i
		}
	}
	return c
}

// index of a column (-1 when absent).
func (c columns) index(names ...string) int {
	for _, name := range names {
		if i, ok := c[name]; ok {
			return i
		}
	}
	// Without regard to case, the first such column in the header.
	best := -1
	for _, name := range names {
		for have, i := range c {
			if strings.EqualFold(have, name) && (best < 0 || i < best) {
				best = i
			}
		}
		if best >= 0 {
			return best
		}
	}
	return -1
}

// field is fields[i] trimmed, or "" when i is -1 or past the row's end.
func field(fields []string, i int) string {
	if i < 0 || i >= len(fields) {
		return ""
	}
	return strings.TrimSpace(fields[i])
}
