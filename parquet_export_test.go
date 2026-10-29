package main

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clonoloom/internal/export"
	"clonoloom/internal/invariant"
	"clonoloom/internal/qc"
)

func TestParquetNativeAndDeidentifiedFixtures(t *testing.T) {
	a, h, deid, path := deidFixture(t)
	before, _ := os.ReadFile(path)
	ds, entry, _, e := a.clonotypeTable(clonotypeRequest{Dataset: deid.Dataset, Definition: deid.Definition, Filter: deid.Filter})
	if e != nil {
		t.Fatal(e)
	}
	names := sampleNames(ds)
	keep, _ := qc.Apply(ds.Result.Table, deid.Filter)
	dir := os.Getenv("CLONOLOOM_PARQUET_FIXTURES")
	save := func(name string, data []byte) {
		t.Helper()
		if dir != "" {
			if e := os.MkdirAll(dir, 0700); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(filepath.Join(dir, name), data, 0600); e != nil {
				t.Fatal(e)
			}
		}
	}
	for _, what := range []string{"clonotypes", "rearrangements"} {
		req := parquetRequest{Dataset: deid.Dataset, What: what, Definition: deid.Definition}
		response := postJSON(t, h, "/api/parquet/export", string(mustJSON(req)))
		if response.Code != 200 {
			t.Fatalf("%d %s", response.Code, response.Body)
		}
		files := unzip(t, response.Body.Bytes())
		if len(files) != 4 || !strings.HasPrefix(files[what+".parquet"], "PAR1") || !strings.Contains(files["NOTICE.txt"], "NOT de-identified") {
			t.Fatal("unsupported bundle shape")
		}
		var receipt parquetReceipt
		json.Unmarshal([]byte(files["inventory.json"]), &receipt)
		if response.Header().Get("Cache-Control") != "no-store" || replayDigest(mustJSON(receipt.Files)) != receipt.Fingerprint {
			t.Fatal("inventory/header")
		}
		for _, f := range receipt.Files {
			if replayDigest([]byte(files[f.Path])) != f.SHA256 || int64(len(files[f.Path])) != f.Size {
				t.Fatal("member integrity")
			}
		}
		var expected bytes.Buffer
		if what == "clonotypes" {
			e = export.Clonotypes(&expected, entry.table, names)
		} else {
			e = export.Rearrangements(&expected, ds.Result.Table, keep, entry.table)
		}
		if e != nil {
			t.Fatal(e)
		}
		save("native-"+what+".zip", response.Body.Bytes())
		save("expected-"+what+".txt", expected.Bytes())
	}
	// Invariant flags and sample shares stay native, with full retained-abundance denominators.
	req := parquetRequest{Dataset: deid.Dataset, What: "clonotypes", Definition: deid.Definition, Species: "human"}
	raw, _, _, e := a.prepareParquet(context.Background(), req)
	if e != nil {
		t.Fatal(e)
	}
	save("native-invariant.zip", raw)
	inv, e := invariant.Compute(entry.table, "human")
	if e != nil {
		t.Fatal(e)
	}
	var withInvariant, sampleInvariant bytes.Buffer
	export.ClonotypesWithInvariant(&withInvariant, entry.table, names, inv)
	export.InvariantSamples(&sampleInvariant, inv, names)
	save("expected-invariant.txt", withInvariant.Bytes())
	save("expected-invariant-samples.txt", sampleInvariant.Bytes())
	deid.StorageFormat = "parquet"
	p, e := a.prepareDeidentified(context.Background(), deid)
	if e != nil {
		t.Fatal(e)
	}
	files := unzip(t, p.Bundle)
	if len(files) != 6 || p.Preview.Record.StorageFormat != "parquet" {
		t.Fatal("deidentified Parquet scope")
	}
	for name, data := range files {
		if strings.Contains(data, "PRIVATE-") || strings.Contains(data, path) {
			t.Fatalf("identity in %s", name)
		}
	}
	save("deidentified-parquet.zip", p.Bundle)
	save("PRIVATE-map.json", p.IdentityMap)
	// Reference tables for independent typed parity use the same reviewed mapping, via map inversion.
	save("deidentified-record.json", mustJSON(p.Preview))
	ds.Credits = []credit{{Title: "Restricted example", LicenseClass: "terms", Citation: "Required citation"}}
	rejected := postJSON(t, h, "/api/parquet/export", string(mustJSON(req)))
	if rejected.Code != 409 || !strings.Contains(rejected.Body.String(), "restricted") {
		t.Fatal("provider policy bypass")
	}
	req.Accept = true
	raw, _, _, e = a.prepareParquet(context.Background(), req)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(unzip(t, raw)["NOTICE.txt"], "THESE DATA MAY NOT BE SHARED") {
		t.Fatal("notice lost")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("source changed")
	}
}
func TestParquetConversionExactCountsNullsAndBounds(t *testing.T) {
	ctx := context.Background()
	provenance := json.RawMessage(`{"unit":"umis","definitionId":"test-control"}`)
	base := "clonotype_id,chain1_locus,chain1_junction,chain1_v,chain1_j,total,samples,total,sample_000001\n"
	raw := []byte(base + "c-1,TRB,CASSF,TRBV5-1,TRBJ1-1,9007199254740991,1,9007199254740991,\n" + "c-2,TRB,CASSF,TRBV5-1,TRBJ1-1,0,1,0,\n")
	out, s, e := nativeParquet(ctx, raw, "clonotypes", []string{"total", "sample_000001"}, provenance, "fixed notice")
	if e != nil {
		t.Fatal(e)
	}
	again, _, e := nativeParquet(ctx, raw, "clonotypes", []string{"total", "sample_000001"}, provenance, "fixed notice")
	if e != nil || !bytes.Equal(out, again) {
		t.Fatal("nondeterministic representation")
	}
	if s.Samples[0].Column != "sample_000001" || s.Samples[0].Sample != "total" || s.Samples[1].Column != "sample_000002" {
		t.Fatal("ambiguous sample names")
	}
	if dir := os.Getenv("CLONOLOOM_PARQUET_FIXTURES"); dir != "" {
		os.MkdirAll(dir, 0700)
		os.WriteFile(filepath.Join(dir, "native-count-boundary.parquet"), out, 0600)
	}
	for _, token := range []string{"9007199254740992", "9007199254740993", "1.00000000000000001", "0.5", "-1", "NaN", "Inf", "1e-999999999"} {
		bad := []byte(strings.ReplaceAll(string(raw), "9007199254740991", token))
		if _, _, e := nativeParquet(ctx, bad, "clonotypes", []string{"total", "sample_000001"}, provenance, "notice"); e == nil {
			t.Fatalf("invalid count %s", token)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, e := nativeParquet(cancelled, raw, "clonotypes", []string{"total", "sample_000001"}, provenance, ""); e == nil {
		t.Fatal("cancellation lost")
	}
	if _, _, e := nativeParquet(ctx, []byte("x\n1\n"), "unmapped", nil, provenance, ""); e == nil {
		t.Fatal("unmapped table")
	}
	// Multiple groups and empty files retain declared schemas; no synthetic rows are introduced.
	large := []byte("subject_id,annotation\n" + strings.Repeat("subject-control,\n", 16400))
	out, s, e = nativeParquet(ctx, large, "samples", nil, provenance, "")
	if e != nil || s.Rows != 16400 {
		t.Fatal(e)
	}
	if dir := os.Getenv("CLONOLOOM_PARQUET_FIXTURES"); dir != "" {
		os.WriteFile(filepath.Join(dir, "multiple-groups.parquet"), out, 0600)
	}
	out, s, e = nativeParquet(ctx, []byte(base), "clonotypes", []string{"total", "sample_000001"}, provenance, "")
	if e != nil || s.Rows != 0 {
		t.Fatal(e)
	}
	if dir := os.Getenv("CLONOLOOM_PARQUET_FIXTURES"); dir != "" {
		os.WriteFile(filepath.Join(dir, "empty-native.parquet"), out, 0600)
	}
}

func TestParquetNativeAIRRCountTypes(t *testing.T) {
	for _, field := range []string{"duplicate_count", "umi_count", "templates"} {
		for _, value := range []string{"", "0", "9007199254740991"} {
			raw := []byte("sequence_id\t" + field + "\nseq-control\t" + value + "\n")
			_, schema, e := nativeParquet(context.Background(), raw, "rearrangements", nil, json.RawMessage(`{}`), "control")
			if e != nil || schema.Fields[1].Type != "INT64" || !schema.Fields[1].Nullable {
				t.Fatalf("%s=%s did not retain nullable INT64: %v", field, value, e)
			}
		}
		for _, value := range []string{"0.5", "-1", "9007199254740992", "NaN", "Inf"} {
			raw := []byte("sequence_id\t" + field + "\nseq-control\t" + value + "\n")
			if _, _, e := nativeParquet(context.Background(), raw, "rearrangements", nil, json.RawMessage(`{}`), "control"); e == nil {
				t.Fatalf("%s=%s accepted instead of refusing invalid count", field, value)
			}
		}
	}
}

func TestParquetRefusesNonfiniteBeforeTextConversion(t *testing.T) {
	a, h, r, _ := deidFixture(t)
	ds, entry, _, e := a.clonotypeTable(clonotypeRequest{Dataset: r.Dataset, Definition: r.Definition})
	if e != nil {
		t.Fatal(e)
	}
	ds.Result.Table.Count[0] = math.Inf(1)
	req := parquetRequest{Dataset: r.Dataset, What: "rearrangements", Definition: r.Definition}
	response := postJSON(t, h, "/api/parquet/export", string(mustJSON(req)))
	if response.Code != 409 || response.Header().Get("Content-Type") == "application/zip" {
		t.Fatal("non-finite count was represented as missing")
	}
	entry.table.Clonotypes[0].Total = math.Inf(1)
	req.What = "clonotypes"
	if response := postJSON(t, h, "/api/parquet/export", string(mustJSON(req))); response.Code != 409 {
		t.Fatal("non-finite aggregate was represented as missing")
	}
	for _, body := range []string{`{"what":"result"}`, `{"what":"clonotypes","unknown":true}`, `{"what":"clonotypes","what":"rearrangements"}`} {
		if response := postJSON(t, h, "/api/parquet/export", body); response.Code == 200 {
			t.Fatal("unmapped request accepted")
		}
	}
}

func FuzzNativeParquet(f *testing.F) {
	f.Add([]byte("clonotype_id,chain1_locus,chain1_junction,chain1_v,chain1_j,total,samples\nc1,TRB,CASSF,V,J,0,0\n"))
	f.Add([]byte("a,b,c,d,e,x,y,z,k,total\n"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 4096 {
			return
		}
		nativeParquet(context.Background(), raw, "clonotypes", nil, json.RawMessage(`{}`), "fuzz control")
	})
}
