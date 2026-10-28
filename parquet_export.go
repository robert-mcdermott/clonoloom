package main

// Native table values are the scientific source; this is a representation change, not a new
// estimator. Apache Parquet/Thrift format, https://parquet.apache.org/docs/file-format/;
// AIRR Rearrangement schema 2.0, Vander Heiden et al. 2018, doi:10.3389/fimmu.2018.02206.
// ClonoLoom's choice: nullable flat fields, explicit sample-column map and conservative count
// conversion. Native float64 counts outside the consecutive exact integer range are refused.
import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"

	"clonoloom/internal/clonotype"
	"clonoloom/internal/parquet"
	"clonoloom/internal/qc"
	"clonoloom/internal/rep"
)

const parquetTableLimit = 64 << 20
const parquetLimits = "Parquet representation alone does not de-identify data or establish scientific replay; the identity policy is recorded in provenance. Empty native fields become null; explicit zero stays zero. Count conversion refuses negative, fractional or non-finite values and values above 9007199254740991; precision already lost in the native float64 model cannot be recovered. Other AIRR evidence remains native text, including invalid annotations. Clonotype sample columns are mapped explicitly; null means no stored abundance, not an invented measurement."

type tableBuffer struct {
	bytes.Buffer
	ctx context.Context
}

func (b *tableBuffer) Write(p []byte) (int, error) {
	if e := b.ctx.Err(); e != nil {
		return 0, e
	}
	if len(p) > parquetTableLimit-b.Len() {
		return 0, errors.New("table export exceeds 64 MiB")
	}
	return b.Buffer.Write(p)
}
func (b *tableBuffer) WriteString(s string) (int, error) { return b.Write([]byte(s)) }

type parquetColumn struct {
	Name       string `json:"name"`
	NativeName string `json:"nativeName"`
	Type       string `json:"type"`
	Nullable   bool   `json:"nullable"`
	Role       string `json:"role"`
}
type parquetSampleColumn struct {
	Column string `json:"column"`
	Sample string `json:"sample"`
}
type parquetSchema struct {
	Format     string                `json:"format"`
	Version    int                   `json:"version"`
	Table      string                `json:"table"`
	Rows       int                   `json:"rows"`
	Fields     []parquetColumn       `json:"fields"`
	Samples    []parquetSampleColumn `json:"sampleColumns,omitempty"`
	Provenance json.RawMessage       `json:"provenance"`
	Limits     string                `json:"limits"`
}

// nativeParquet parses only known native table layouts, never arbitrary result JSON or nested data.
func nativeParquet(ctx context.Context, raw []byte, kind string, samples []string, provenance json.RawMessage, notice string) ([]byte, parquetSchema, error) {
	schema := parquetSchema{Format: "clonoloom-parquet-table", Version: 1, Table: kind, Provenance: provenance, Limits: parquetLimits}
	fail := func(e error) ([]byte, parquetSchema, error) { return nil, schema, e }
	if len(raw) > parquetTableLimit || !utf8.Valid(raw) || !json.Valid(provenance) {
		return fail(errors.New("invalid or oversized native table/provenance"))
	}
	var read func() ([]string, error)
	if kind == "rearrangements" {
		scan := bufio.NewScanner(bytes.NewReader(raw))
		scan.Buffer(make([]byte, 64<<10), 8<<20)
		read = func() ([]string, error) {
			if !scan.Scan() {
				if e := scan.Err(); e != nil {
					return nil, e
				}
				return nil, io.EOF
			}
			return strings.Split(scan.Text(), "\t"), nil
		}
	} else {
		reader := csv.NewReader(bytes.NewReader(raw))
		read = reader.Read
	}
	header, e := read()
	if e != nil {
		return fail(e)
	}
	if len(header) == 0 || len(header) > 1100 {
		return fail(errors.New("unsupported column count"))
	}
	sampleStart, sampleEnd := -1, -1
	switch kind {
	case "clonotypes":
		if len(samples) > 1000 || len(header) < 7 || header[0] != "clonotype_id" {
			return fail(errors.New("unsupported clonotype schema"))
		}
		for i := 5; i+1 < len(header) && i <= 9; i += 4 {
			if header[i] == "total" && header[i+1] == "samples" {
				sampleStart = i + 2
				break
			}
		}
		sampleEnd = sampleStart + len(samples)
		if sampleStart < 0 || sampleEnd > len(header) {
			return fail(errors.New("sample columns do not match"))
		}
		for i, name := range samples {
			if header[sampleStart+i] != name {
				return fail(errors.New("sample column order differs"))
			}
		}
	case "rearrangements", "samples", "invariant-samples":
	default:
		return fail(errors.New("no Parquet schema for this table kind"))
	}
	fields := make([]parquet.Field, len(header))
	for i, name := range header {
		f := parquet.Field{Name: name, Type: parquet.ByteArray, Annotation: "string", Optional: true}
		role := "native_text"
		switch {
		case kind == "clonotypes" && i >= sampleStart && i < sampleEnd:
			f.Name = fmt.Sprintf("sample_%06d", i-sampleStart+1)
			f.Type = parquet.Int64
			f.Annotation = ""
			role = "sample_abundance"
			schema.Samples = append(schema.Samples, parquetSampleColumn{f.Name, name})
		case kind == "clonotypes" && (name == "total" || name == "samples") || kind == "rearrangements" && (name == "duplicate_count" || name == "umi_count" || name == "templates") || kind == "invariant-samples" && (name == "total" || name == "evaluable" || name == "matched" || name == "rule_version"):
			f.Type = parquet.Int64
			f.Annotation = ""
			role = "integer_count"
		case kind == "rearrangements" && (name == "productive" || name == "vj_in_frame" || name == "stop_codon"):
			f.Type = parquet.Boolean
			f.Annotation = ""
			role = "native_flag"
		case kind == "invariant-samples" && name == "share":
			f.Type = parquet.Double
			f.Annotation = ""
			role = "fraction_of_total_retained_abundance"
		}
		fields[i] = f
		schema.Fields = append(schema.Fields, parquetColumn{f.Name, name, f.Type.String(), true, role})
	}
	out := &tableBuffer{ctx: ctx}
	writer, e := parquet.NewWriter(ctx, out, fields, "ClonoLoom "+version+" table schema 1")
	if e != nil {
		return fail(e)
	}
	columns := make([]*parquet.Values, len(fields))
	reset := func() {
		for i := range columns {
			columns[i] = &parquet.Values{}
		}
	}
	reset()
	groupRows, groupBytes := 0, 0
	flush := func() error {
		if groupRows == 0 {
			return nil
		}
		if e := writer.WriteRowGroup(columns); e != nil {
			return e
		}
		reset()
		groupRows = 0
		groupBytes = 0
		return nil
	}
	for {
		if e = ctx.Err(); e != nil {
			return fail(e)
		}
		row, err := read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fail(err)
		}
		if len(row) != len(fields) {
			return fail(errors.New("native row width differs"))
		}
		rowBytes := 0
		for _, s := range row {
			rowBytes += len(s) + 16
			if len(s) > 1<<20 {
				return fail(errors.New("table value exceeds 1 MiB"))
			}
		}
		if groupRows >= 8192 || groupBytes+rowBytes > 16<<20 {
			if e = flush(); e != nil {
				return fail(e)
			}
		}
		for i, text := range row {
			c := columns[i]
			valid := text != ""
			c.Valid = append(c.Valid, valid)
			switch fields[i].Type {
			case parquet.ByteArray:
				c.Bytes = append(c.Bytes, []byte(text))
			case parquet.Boolean:
				if valid && text != "T" && text != "F" {
					return fail(fmt.Errorf("row %d field %s: expected a native T/F flag", schema.Rows+1, header[i]))
				}
				c.Bool = append(c.Bool, text == "T")
			case parquet.Int64:
				var value int64
				if valid {
					if len(text) > 64 {
						return fail(errors.New("count token exceeds 64 bytes"))
					}
					if at := strings.IndexAny(text, "eE"); at >= 0 {
						exponent, err := strconv.ParseInt(text[at+1:], 10, 16)
						if err != nil || exponent < -400 || exponent > 400 {
							return fail(errors.New("count exponent exceeds native range"))
						}
					}
					v, ok := new(big.Rat).SetString(text)
					if !ok || !v.IsInt() || v.Sign() < 0 || v.Num().BitLen() > 53 {
						return fail(fmt.Errorf("row %d field %s: count is not an exact nonnegative native integer at most 9007199254740991", schema.Rows+1, header[i]))
					}
					value = v.Num().Int64()
				}
				c.Int64 = append(c.Int64, value)
			case parquet.Double:
				var value float64
				if valid {
					value, e = strconv.ParseFloat(text, 64)
					if e != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
						return fail(errors.New("invalid native invariant share"))
					}
				}
				c.Float64 = append(c.Float64, value)
			}
		}
		groupRows++
		groupBytes += rowBytes
		schema.Rows++
	}
	if e = flush(); e != nil {
		return fail(e)
	}
	if e = writer.SetKeyValue("clonoloom", string(mustJSON(schema))); e != nil {
		return fail(e)
	}
	if e = writer.SetKeyValue("NOTICE", notice); e != nil {
		return fail(e)
	}
	if e = writer.Close(); e != nil {
		return fail(e)
	}
	return out.Bytes(), schema, nil
}

// The text exporter represents non-finite values as blank; catch infinities before that step.
// NaN is the native missing sentinel, not a measurement to be coerced to zero.
func checkParquetCounts(ctx context.Context, rt *rep.Table, ct *clonotype.Table, filter qc.Filter, rearrangements bool) error {
	check := func(v float64) error {
		if math.IsInf(v, 0) {
			return errors.New("non-finite native count cannot be exported as Parquet")
		}
		return nil
	}
	for i, c := range ct.Clonotypes {
		if i%1024 == 0 {
			if e := ctx.Err(); e != nil {
				return e
			}
		}
		if e := check(c.Total); e != nil {
			return e
		}
		_, values := ct.Abundances(i)
		for _, v := range values {
			if e := check(v); e != nil {
				return e
			}
		}
	}
	if rearrangements {
		keep, _ := qc.Apply(rt, filter)
		for row, yes := range keep {
			if !yes {
				continue
			}
			if row%1024 == 0 {
				if e := ctx.Err(); e != nil {
					return e
				}
			}
			if e := check(rt.Count[row]); e != nil {
				return e
			}
			for _, column := range [][]float64{rt.Reads, rt.UMIs} {
				if column != nil {
					if e := check(column[row]); e != nil {
						return e
					}
				}
			}
		}
	}
	return ctx.Err()
}
