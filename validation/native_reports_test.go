package validation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"clonoloom/internal/reads"
)

func TestNativeReportsIndependentValues(t *testing.T) {
	var expected map[string]struct {
		Format, Unit, Header, SHA256 string
		Columns                      map[string][]string
		NativeRows                   []string `json:"native_rows"`
	}
	raw, err := os.ReadFile("reference/native_reports.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &expected); err != nil {
		t.Fatal(err)
	}
	for file, want := range expected {
		t.Run(file, func(t *testing.T) {
			r, err := reads.ReadFiles(context.Background(), []string{filepath.Join("../testdata/native-reports", file)})
			if err != nil {
				t.Fatal(err)
			}
			s := r.Sources[0]
			if string(s.Format) != want.Format || string(r.Unit) != want.Unit || s.SHA256 != want.SHA256 || s.Native.Header != want.Header {
				t.Fatal("source contract differs")
			}
			got := columns(r.Table)
			for i := 0; i < r.Table.Len(); i++ {
				got["vj_in_frame"] = append(got["vj_in_frame"], flagText(r.Table.VJInFrame[i]))
				got["stop_codon"] = append(got["stop_codon"], flagText(r.Table.StopCodon[i]))
				got["source_line"] = append(got["source_line"], strconv.Itoa(int(r.Table.Line[i])))
				if s.Native.Rows[i].Text != want.NativeRows[i] || s.Native.Rows[i].Line != r.Table.Line[i] {
					t.Fatalf("source row %d changed", i)
				}
			}
			if !reflect.DeepEqual(got, want.Columns) {
				for k, v := range want.Columns {
					if !reflect.DeepEqual(v, got[k]) {
						t.Errorf("%s: got %q want %q", k, got[k], v)
					}
				}
			}
		})
	}
}
