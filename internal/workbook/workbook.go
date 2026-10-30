// Package workbook writes bounded, formula-free SpreadsheetML workbooks.
// Format references: Microsoft Learn, Structure of a SpreadsheetML document;
// Microsoft Support, Excel specifications and limits. See research §5.66.
// Follows the Apache-2.0 CytoWeave workbook presentation pattern; native values
// and exact JSON tokens remain in Go and never pass through browser numbers.
package workbook

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"clonoloom/internal/airr"
)

const MaxNodes = 100000
const MaxBytes = 64 << 20
const Limits = "Static evidence, not spreadsheet recomputation or a scientific validity certificate. Excel numbers have at most 15 significant decimal digits and use binary floating point. Precision-sensitive, out-of-range and negative-zero numeric tokens are text. The Exact JSON scalar column is always text and authoritative; native JSON companions retain original bytes. Strings remain strings, including numeric-looking IDs and formula-like text. Null, empty strings, empty containers and absent paths are distinct. All recorded values, denominators and exclusions are retained; worksheet rows are JSON nodes, not independent subjects. No formulas, macros, external links or inferred dates. No de-identification or redistribution permission is conferred."

type Cell struct{ Type, Text string }
type Sheet struct {
	Name   string
	Rows   [][]Cell
	Widths []int
}
type Stats struct {
	Nodes       int `json:"nodes"`
	NumericText int `json:"numericText"`
}

func Text(s string) Cell { return Cell{"text", s} }

// Numeric permits ordinary Excel numeric use conservatively. The exact token is
// retained independently even for admitted numbers (decimal fractions are not
// generally exact in binary). Never infer types from numeric-looking strings.
func Numeric(s string) bool {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) || (f == 0 && strings.HasPrefix(s, "-")) {
		return false
	}
	mantissa := strings.FieldsFunc(s, func(r rune) bool { return r == 'e' || r == 'E' })[0]
	digits := strings.Trim(strings.ReplaceAll(strings.TrimPrefix(mantissa, "-"), ".", ""), "0")
	if len(digits) > 15 || f == 0 && digits != "" {
		return false
	}
	return f == 0 || math.Abs(f) >= 1e-300 && math.Abs(f) <= 1e300
}

// Evidence is a lossless typed node index: every container and scalar gets one
// row, with RFC 6901 pointers. No inferred joins, flattening of biological units,
// absent-to-null conversion, row sampling or scientific computation occurs.
func Evidence(ctx context.Context, raw []byte, name string, budget *int) (Sheet, Stats, error) {
	sheet := Sheet{Name: name, Widths: []int{62, 18, 42, 52, 14}, Rows: [][]Cell{{Text("JSON pointer"), Text("JSON node type"), Text("Spreadsheet value"), Text("Exact JSON scalar"), Text("Child count")}}}
	stats := Stats{}
	if len(raw) > 32<<20 {
		return sheet, stats, errors.New("one evidence document exceeds 32 MiB")
	}
	if err := airr.CheckExactJSON(ctx, raw); err != nil {
		return sheet, stats, err
	}
	var visit func(json.RawMessage, string) error
	visit = func(value json.RawMessage, path string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if *budget <= 0 {
			return errors.New("workbook exceeds 100,000 JSON nodes; select fewer results")
		}
		*budget--
		stats.Nodes++
		value = bytes.TrimSpace(value)
		row := []Cell{Text(path), Text(""), {}, {}, {}}
		switch value[0] {
		case '{':
			var obj map[string]json.RawMessage
			json.Unmarshal(value, &obj)
			row[1] = Text("object")
			row[4] = Cell{"number", strconv.Itoa(len(obj))}
			sheet.Rows = append(sheet.Rows, row)
			keys := []string{}
			for key := range obj {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				escaped := strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
				if err := visit(obj[key], path+"/"+escaped); err != nil {
					return err
				}
			}
		case '[':
			var items []json.RawMessage
			json.Unmarshal(value, &items)
			row[1] = Text("array")
			row[4] = Cell{"number", strconv.Itoa(len(items))}
			sheet.Rows = append(sheet.Rows, row)
			for i, item := range items {
				if err := visit(item, path+"/"+strconv.Itoa(i)); err != nil {
					return err
				}
			}
		default:
			token := string(value)
			row[3] = Text(token)
			switch value[0] {
			case '"':
				var s string
				json.Unmarshal(value, &s)
				row[1] = Text("string")
				row[2] = Text(s)
			case 'n':
				row[1] = Text("null")
			case 't', 'f':
				row[1] = Text("boolean")
				row[2] = Cell{"boolean", token}
			default:
				row[1] = Text("number")
				row[2] = Text(token)
				if Numeric(token) {
					row[2] = Cell{"number", token}
				} else {
					stats.NumericText++
				}
			}
			sheet.Rows = append(sheet.Rows, row)
		}
		return nil
	}
	err := visit(raw, "")
	return sheet, stats, err
}

var escapes = regexp.MustCompile(`_[xX][0-9a-fA-F]{4}_`)

func escaped(s string) (string, error) {
	if !utf8.ValidString(s) || len(utf16.Encode([]rune(s))) > 32767 || strings.Count(s, "\n") > 253 {
		return "", errors.New("cell exceeds Excel text/line limits or is not UTF-8")
	}
	// Escape literal OOXML escape-looking strings before encoding control codes.
	s = escapes.ReplaceAllStringFunc(s, func(v string) string { return "_x005F_" + v[1:] })
	var safe strings.Builder
	for _, r := range s {
		if r < 32 && r != '\t' && r != '\n' || r == 0xfffe || r == 0xffff {
			fmt.Fprintf(&safe, "_x%04X_", r)
		} else {
			safe.WriteRune(r)
		}
	}
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(safe.String()))
	return b.String(), nil
}
func column(n int) string {
	s := ""
	for n++; n > 0; n = (n - 1) / 26 {
		s = string(rune('A'+(n-1)%26)) + s
	}
	return s
}

const mainNS = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
const relNS = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
const header = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`
const styles = `<styleSheet xmlns="` + mainNS + `"><fonts count="2"><font><sz val="11"/><name val="Calibri"/></font><font><b/><color rgb="FFFFFFFF"/><sz val="11"/><name val="Calibri"/></font></fonts><fills count="3"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill><fill><patternFill patternType="solid"><fgColor rgb="FF243549"/><bgColor indexed="64"/></patternFill></fill></fills><borders count="2"><border><left/><right/><top/><bottom/><diagonal/></border><border><left style="thin"><color rgb="FFE1E5EA"/></left><right style="thin"><color rgb="FFE1E5EA"/></right><top/><bottom style="hair"><color rgb="FFE1E5EA"/></bottom><diagonal/></border></borders><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="3"><xf numFmtId="0" fontId="0" fillId="0" borderId="1" xfId="0" applyAlignment="1" applyBorder="1"><alignment vertical="top"/></xf><xf numFmtId="0" fontId="1" fillId="2" borderId="0" xfId="0" applyAlignment="1" applyBorder="1"><alignment vertical="center" wrapText="1"/></xf><xf numFmtId="49" fontId="0" fillId="0" borderId="1" xfId="0" applyAlignment="1" applyBorder="1"><alignment vertical="top" wrapText="1"/></xf></cellXfs><cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles></styleSheet>`

func Write(ctx context.Context, sheets []Sheet) ([]byte, error) {
	if len(sheets) < 1 || len(sheets) > 20 {
		return nil, errors.New("workbook requires 1–20 sheets")
	}
	parts := map[string]string{"xl/styles.xml": header + styles}
	var book, rels, types strings.Builder
	book.WriteString(header + `<workbook xmlns="` + mainNS + `" xmlns:r="` + relNS + `"><bookViews><workbookView activeTab="0"/></bookViews><sheets>`)
	rels.WriteString(header + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	types.WriteString(header + `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>`)
	seen := map[string]bool{}
	total := 0
	cells := 0
	for i, s := range sheets {
		if len(s.Name) == 0 || len(utf16.Encode([]rune(s.Name))) > 31 || strings.ContainsAny(s.Name, `[]:*?/\`) || strings.HasPrefix(s.Name, "'") || strings.HasSuffix(s.Name, "'") || strings.EqualFold(s.Name, "history") || seen[strings.ToLower(s.Name)] {
			return nil, errors.New("invalid or repeated worksheet name")
		}
		seen[strings.ToLower(s.Name)] = true
		name, err := escaped(s.Name)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&book, `<sheet name="%s" sheetId="%d" r:id="s%d"/>`, name, i+1, i+1)
		fmt.Fprintf(&rels, `<Relationship Id="s%d" Type="%s/worksheet" Target="worksheets/sheet%d.xml"/>`, i+1, relNS, i+1)
		fmt.Fprintf(&types, `<Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, i+1)
		var b strings.Builder
		width := 0
		for _, row := range s.Rows {
			width = max(width, len(row))
		}
		if width < 1 || width > 32 || len(s.Rows) > 100001 {
			return nil, errors.New("worksheet exceeds bounded row/column limits")
		}
		end := column(width-1) + strconv.Itoa(len(s.Rows))
		b.WriteString(header + `<worksheet xmlns="` + mainNS + `"><sheetPr><pageSetUpPr fitToPage="1"/></sheetPr><dimension ref="A1:` + end + `"/><sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/><selection pane="bottomLeft"/></sheetView></sheetViews><sheetFormatPr defaultRowHeight="30"/><cols>`)
		for j := 0; j < width; j++ {
			w := 24
			if j < len(s.Widths) {
				w = s.Widths[j]
			}
			if w < 8 || w > 100 {
				return nil, errors.New("invalid column width")
			}
			fmt.Fprintf(&b, `<col min="%d" max="%d" width="%d" customWidth="1"/>`, j+1, j+1, w)
		}
		b.WriteString(`</cols><sheetData>`)
		for row, values := range s.Rows {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			cells += len(values)
			if cells > 600000 {
				return nil, errors.New("workbook exceeds 600,000 cells")
			}
			height := 30
			for j, c := range values {
				width := 24
				if j < len(s.Widths) {
					width = s.Widths[j]
				}
				lines := 0
				for _, line := range strings.Split(c.Text, "\n") {
					lines += max(1, (len([]rune(line))+width-1)/width)
				}
				height = max(height, min(409, (lines+1)*15))
			}
			if row == 0 {
				height = 32
			}
			fmt.Fprintf(&b, `<row r="%d" ht="%d" customHeight="1">`, row+1, height)
			for j, c := range values {
				if c.Type == "" {
					fmt.Fprintf(&b, `<c r="%s" s="0"/>`, column(j)+strconv.Itoa(row+1))
					continue
				}
				ref := column(j) + strconv.Itoa(row+1)
				style := 0
				if c.Type == "text" {
					style = 2
				}
				if row == 0 {
					style = 1
				}
				switch c.Type {
				case "text":
					text, err := escaped(c.Text)
					if err != nil {
						return nil, fmt.Errorf("%s!%s: %w", s.Name, ref, err)
					}
					fmt.Fprintf(&b, `<c r="%s" s="%d" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, ref, style, text)
				case "number":
					if !json.Valid([]byte(c.Text)) || !Numeric(c.Text) {
						return nil, errors.New("unsafe numeric cell")
					}
					fmt.Fprintf(&b, `<c r="%s" s="%d" t="n"><v>%s</v></c>`, ref, style, c.Text)
				case "boolean":
					if c.Text != "true" && c.Text != "false" {
						return nil, errors.New("invalid boolean cell")
					}
					v := 0
					if c.Text == "true" {
						v = 1
					}
					fmt.Fprintf(&b, `<c r="%s" s="%d" t="b"><v>%d</v></c>`, ref, style, v)
				default:
					return nil, errors.New("unsupported cell type")
				}
			}
			b.WriteString(`</row>`)
			if total+b.Len() > MaxBytes {
				return nil, errors.New("workbook XML exceeds 64 MiB")
			}
		}
		b.WriteString(`</sheetData><autoFilter ref="A1:` + end + `"/><pageMargins left="0.5" right="0.5" top="0.5" bottom="0.5" header="0.3" footer="0.3"/><pageSetup paperSize="8" orientation="landscape" fitToWidth="1" fitToHeight="0"/></worksheet>`)
		total += b.Len()
		parts[fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1)] = b.String()
	}
	book.WriteString(`</sheets><definedNames>`)
	for i, s := range sheets {
		name, _ := escaped("'" + strings.ReplaceAll(s.Name, "'", "''") + "'!$1:$1")
		fmt.Fprintf(&book, `<definedName name="_xlnm.Print_Titles" localSheetId="%d">%s</definedName>`, i, name)
	}
	book.WriteString(`</definedNames></workbook>`)
	fmt.Fprintf(&rels, `<Relationship Id="styles" Type="%s/styles" Target="styles.xml"/></Relationships>`, relNS)
	types.WriteString(`</Types>`)
	parts["xl/workbook.xml"] = book.String()
	parts["xl/_rels/workbook.xml.rels"] = rels.String()
	parts["[Content_Types].xml"] = types.String()
	parts["_rels/.rels"] = header + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="workbook" Type="` + relNS + `/officeDocument" Target="xl/workbook.xml"/></Relationships>`
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	names := []string{}
	for name := range parts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		w, err := z.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err = w.Write([]byte(parts[name])); err != nil {
			return nil, err
		}
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
