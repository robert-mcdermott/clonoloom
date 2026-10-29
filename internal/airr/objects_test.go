package airr

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestAIRRObjectsExactAndLinked(t *testing.T) {
	ctx := context.Background()
	j, _ := os.ReadFile("../../testdata/airr-objects/study.json")
	y, _ := os.ReadFile("../../testdata/airr-objects/study.yaml")
	expected, _ := os.ReadFile("../../testdata/airr-objects/study.expected.json")
	a, err := ReadDataFile(ctx, j, false)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ReadDataFile(ctx, y, true)
	if err != nil {
		t.Fatal(err)
	}
	c, err := ReadDataFile(ctx, expected, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Objects) != 7 || len(a.Repertoires) != 1 {
		t.Fatal(a)
	}
	var export []map[string]any
	for i, o := range a.Objects {
		if !strings.Contains(string(j), string(o.Raw)) {
			t.Fatal("original object bytes changed")
		}
		var x, z any
		dx := json.NewDecoder(strings.NewReader(string(o.Raw)))
		dx.UseNumber()
		dx.Decode(&x)
		dz := json.NewDecoder(strings.NewReader(string(b.Objects[i].Raw)))
		dz.UseNumber()
		dz.Decode(&z)
		var reference any
		dr := json.NewDecoder(strings.NewReader(string(c.Objects[i].Raw)))
		dr.UseNumber()
		dr.Decode(&reference)
		bo, co := b.Objects[i], c.Objects[i]
		bo.Raw, co.Raw = nil, nil
		if !reflect.DeepEqual(x, z) || !reflect.DeepEqual(z, reference) || !reflect.DeepEqual(bo, co) {
			t.Fatalf("exact independent YAML reference differs: %s", o.Path)
		}
		export = append(export, map[string]any{"kind": o.Kind, "path": o.Path, "rawJSON": string(o.Raw)})
	}
	if path := os.Getenv("CLONOLOOM_OBJECT_EVIDENCE"); path != "" {
		v, _ := json.Marshal(export)
		if err := os.WriteFile(path, v, 0600); err != nil {
			t.Fatal(err)
		}
	}
	statuses := map[string]string{}
	for _, o := range a.Objects {
		for _, l := range o.Links {
			statuses[l.Path] = l.Status
		}
	}
	for path, want := range map[string]string{"/Cell/0/receptors/0": "loaded", "/Cell/1/receptors/0": "unresolved", "/Cell/1/data_processing_id": "unresolved", "/Clone/0/nodes/0": "loaded", "/Node/0/sequence_id": "not-assessed"} {
		if statuses[path] != want {
			t.Fatalf("%s: %s want %s", path, statuses[path], want)
		}
	}
}
func TestAIRRObjectsRefuseAmbiguity(t *testing.T) {
	wrap := func(s string) []byte { return []byte(`{"Info":{"version":"2.0"},` + s + `}`) }
	bad := []string{
		`"Cell":null`, `"Cell":[null]`, `"Cell":[{"cell_id":"c","repertoire_id":null}]`,
		`"Cell":[{"cell_id":1,"repertoire_id":null,"virtual_pairing":false}]`,
		`"Cell":[{"cell_id":" c","repertoire_id":null,"virtual_pairing":false}]`,
		`"Cell":[{"cell_id":"c","repertoire_id":null,"virtual_pairing":"false"}]`,
		`"Cell":[{"cell_id":"c","repertoire_id":null,"virtual_pairing":false,"receptors":["r","r"]}]`,
		`"Node":[{"node_id":"n","repertoire_id":"r","cell_id":"c","sequence_id":"s"}]`,
		`"Clone":[{"clone_id":"c","repertoire_id":"r","repertoire_group_id":"g","nodes":[]}]`,
		`"Clone":[{"clone_id":"c","repertoire_id":"r","nodes":["n"]}]`,
		`"Clone":[{"clone_id":"c","repertoire_id":"r","nodes":[],"clone_count":1.5}]`,
		`"Node":[{"node_id":"n","repertoire_id":null},{"node_id":"n","repertoire_id":null}]`,
		`"Cell":[{"cell_id":"c","repertoire_id":"a","virtual_pairing":null}],"Node":[{"node_id":"n","repertoire_id":"b","cell_id":"c"}]`,
		`"Clone":[{"clone_id":"c","repertoire_id":"a","nodes":[{"node_id":"n","repertoire_id":"b"}]}]`,
		`"Receptor":[{"receptor_id":"r"}]`,
		`"Repertoire":[{"repertoire_id":"r","data_processing":[{"data_processing_id":"p"},{"data_processing_id":"p"}]}],"Cell":[{"cell_id":"c","repertoire_id":"r","virtual_pairing":null}]`,
	}
	for _, s := range bad {
		if _, err := ReadDataFile(context.Background(), wrap(s), false); err == nil {
			t.Fatalf("accepted %s", s)
		}
	}
	source, _ := os.ReadFile("../../testdata/airr-objects/study.json")
	for _, s := range []string{strings.Replace(string(source), `"version": "2.0"`, `"version": "1.4"`, 1), strings.Replace(string(source), `"receptor_hash": "`, `"receptor_hash": "bad`, 1), strings.Replace(string(source), `"TRB"`, `"TRA"`, 1)} {
		if _, err := ReadDataFile(context.Background(), []byte(s), false); err == nil {
			t.Fatal("accepted unsupported version/hash/pairing")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ReadDataFile(ctx, source, false); err == nil {
		t.Fatal("ignored cancellation")
	}
	many := `"Cell":[{"cell_id":"c","repertoire_id":null,"virtual_pairing":false,"receptors":[` + strings.TrimSuffix(strings.Repeat(`"r",`, 1001), ",") + `]}]`
	if _, err := ReadDataFile(context.Background(), wrap(many), false); err == nil {
		t.Fatal("link bound")
	}
}
func FuzzAIRRObjects(f *testing.F) {
	f.Add([]byte(`{"Info":{"version":"2.0"},"Cell":[{"cell_id":"c","repertoire_id":null,"virtual_pairing":false}]}`))
	f.Add([]byte(`{"Info":{"version":"2.0"},"Clone":[]}`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 1<<16 {
			t.Skip()
		}
		d, err := ReadDataFile(context.Background(), raw, false)
		if err != nil {
			return
		}
		if len(d.Objects) > 100000 {
			t.Fatal("unbounded")
		}
		for _, o := range d.Objects {
			if !strings.Contains(string(raw), string(o.Raw)) {
				t.Fatal("evidence changed")
			}
		}
	})
}

func TestAIRRObjectsNeverMisreadBareCellsAsRepertoires(t *testing.T) {
	for _, raw := range []string{`{"cell_id":"c","repertoire_id":"r","virtual_pairing":false}`, `[{"clone_id":"c","repertoire_id":"r","nodes":null}]`} {
		if _, err := ReadDataFile(context.Background(), []byte(raw), false); err == nil {
			t.Fatal("ambiguous bare object accepted as a repertoire")
		}
	}
	if _, err := ReadDataFile(context.Background(), []byte(`{"Repertoire":[{"repertoire_id":"r","cell_id":"legacy extension"}]}`), false); err != nil {
		t.Fatal(err)
	}
}

func TestAIRRReceptorAlphabetIsASCII(t *testing.T) {
	for _, domain := range []string{"acsx", "ACſX"} {
		hash := sha256.Sum256([]byte(strings.ToUpper(domain) + "ACDX"))
		fields := map[string]json.RawMessage{}
		for key, value := range map[string]string{
			"receptor_type": "TCR", "receptor_variable_domain_1_locus": "TRB", "receptor_variable_domain_2_locus": "TRA",
			"receptor_variable_domain_1_aa": domain, "receptor_variable_domain_2_aa": "ACDX", "receptor_hash": hex.EncodeToString(hash[:]),
		} {
			fields[key], _ = json.Marshal(value)
		}
		err := checkReceptor(fields)
		if (err == nil) != (domain == "acsx") {
			t.Fatalf("domain %q: %v", domain, err)
		}
	}
}
