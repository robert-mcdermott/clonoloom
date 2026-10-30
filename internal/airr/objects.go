package airr

// AIRR Standards 2.0 Clone, Single-cell and Receptor schemas:
// https://docs.airr-community.org/en/v2.0.0/datarep/clone.html
// https://docs.airr-community.org/en/v2.0.0/datarep/cell.html
// https://docs.airr-community.org/en/v2.0.0/datarep/receptor.html
// Imported assertions remain separate from ClonoLoom's computed clonotypes.
import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

type ObjectLink struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	ID     string `json:"id"`
	Status string `json:"status"`
}
type Object struct {
	Kind         string          `json:"kind"`
	ID           string          `json:"id"`
	Path         string          `json:"path"`
	RepertoireID string          `json:"repertoireId,omitempty"`
	FileIndex    int             `json:"fileIndex"`
	Links        []ObjectLink    `json:"links"`
	Raw          json.RawMessage `json:"-"`
}
type DataFile struct {
	RepertoireDocument
	Objects []Object
}

// ReadDataFile extends, rather than changes, legacy repertoire input. Only new
// object collections require a declared schema 2.0/2.0.0. YAML Raw is a projection.
func ReadDataFile(ctx context.Context, data []byte, yaml bool) (DataFile, error) {
	var out DataFile
	var err error
	if yaml {
		data, err = decodeMetadataYAML(ctx, data)
		if err != nil {
			return out, err
		}
	}
	if len(data) > maxMetadataBytes {
		return out, fmt.Errorf("AIRR Data File exceeds 64 MiB")
	}
	if err = checkMetadataJSON(ctx, data); err != nil {
		return out, err
	}
	var top map[string]json.RawMessage
	_ = json.Unmarshal(data, &top)
	hasObjects := false
	for _, kind := range []string{"Clone", "Cell", "Receptor", "Node"} {
		if _, ok := top[kind]; ok {
			hasObjects = true
		}
	}
	if !hasObjects {
		// A bare Cell or Clone also has repertoire_id. Do not silently treat
		// that reference as the identity of a legacy Repertoire record.
		if _, wrapped := top["Repertoire"]; !wrapped {
			var candidates []json.RawMessage
			if bytes.TrimSpace(data)[0] == '[' {
				_ = json.Unmarshal(data, &candidates)
			} else {
				candidates = []json.RawMessage{data}
			}
			for _, candidate := range candidates {
				if err := ctx.Err(); err != nil {
					return out, err
				}
				var fields map[string]json.RawMessage
				_ = json.Unmarshal(candidate, &fields)
				for _, key := range []string{"clone_id", "cell_id", "receptor_id", "node_id"} {
					if _, ok := fields[key]; ok {
						return out, fmt.Errorf("bare record has %s: use an explicit AIRR Data File collection and Info.version; wrap legacy Repertoire records with such extension keys in Repertoire", key)
					}
				}
			}
		}
		out.RepertoireDocument, err = ReadRepertoireDocument(ctx, data)
		return out, err
	}
	var info struct {
		Version string `json:"version"`
	}
	if err = json.Unmarshal(top["Info"], &info); err != nil || (info.Version != "2.0" && info.Version != "2.0.0") {
		return out, fmt.Errorf("Clone/Cell/Receptor/Node require Info.version 2.0 or 2.0.0; migrate older schemas explicitly")
	}
	out.SchemaVersion = info.Version
	if _, ok := top["Repertoire"]; ok {
		out.RepertoireDocument, err = ReadRepertoireDocument(ctx, data)
		if err != nil {
			return DataFile{}, err
		}
	}
	var add func(string, string, json.RawMessage) error
	add = func(kind, path string, raw json.RawMessage) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if len(out.Objects) >= 100000 {
			return fmt.Errorf("AIRR Data File exceeds 100000 objects")
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
			return fmt.Errorf("%s must be an object", path)
		}
		o := Object{Kind: kind, Path: path, Raw: raw, Links: []ObjectLink{}}
		idKey := map[string]string{"Clone": "clone_id", "Cell": "cell_id", "Receptor": "receptor_id", "Node": "node_id"}[kind]
		id, err := objectID(fields, idKey, true, false)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		o.ID = id
		link := func(key, target string, required bool) error {
			id, err := objectID(fields, key, required, true)
			if err != nil {
				return err
			}
			if id != "" {
				o.Links = append(o.Links, ObjectLink{path + "/" + key, target, id, "unresolved"})
				if target == "Repertoire" {
					o.RepertoireID = id
				}
			}
			return nil
		}
		if kind != "Receptor" {
			if err = link("repertoire_id", "Repertoire", kind != "Clone"); err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			if kind != "Node" {
				if err = link("data_processing_id", "DataProcessing", false); err != nil {
					return fmt.Errorf("%s: %w", path, err)
				}
			}
		}
		switch kind {
		case "Clone":
			if err = link("repertoire_group_id", "RepertoireGroup", false); err != nil {
				return err
			}
			group, _ := objectID(fields, "repertoire_group_id", false, true)
			if (group == "") == (o.RepertoireID == "") {
				return fmt.Errorf("%s: Clone needs exactly one non-null repertoire_id or repertoire_group_id", path)
			}
			if err = link("inferred_ancestor", "Node", false); err != nil {
				return err
			}
			nodes, ok := fields["nodes"]
			if !ok {
				return fmt.Errorf("%s: nodes is required (may be null)", path)
			}
			if !bytes.Equal(nodes, []byte("null")) {
				var list []json.RawMessage
				if len(nodes) == 0 || nodes[0] != '[' || json.Unmarshal(nodes, &list) != nil {
					return fmt.Errorf("%s/nodes must be an array of Node objects or null", path)
				}
				if len(list) > 1000 {
					return fmt.Errorf("%s exceeds 1000 node links", path)
				}
				for i, node := range list {
					np := fmt.Sprintf("%s/nodes/%d", path, i)
					if err = add("Node", np, node); err != nil {
						return err
					}
					n := out.Objects[len(out.Objects)-1]
					if o.RepertoireID != "" && n.RepertoireID != "" && o.RepertoireID != n.RepertoireID {
						return fmt.Errorf("%s: node repertoire conflicts with Clone", np)
					}
					o.Links = append(o.Links, ObjectLink{np, "Node", n.ID, "unresolved"})
				}
			}
			if count, ok := fields["clone_count"]; ok && !bytes.Equal(count, []byte("null")) {
				s := string(count)
				if s == "" || strings.ContainsAny(s, "-.eE\"") || strings.Trim(s, "0123456789") != "" {
					return fmt.Errorf("%s/clone_count must be a nonnegative integer token or null", path)
				}
			}
		case "Node":
			if err = link("cell_id", "Cell", false); err != nil {
				return err
			}
			if err = link("sequence_id", "Rearrangement", false); err != nil {
				return err
			}
			cell, _ := objectID(fields, "cell_id", false, true)
			seq, _ := objectID(fields, "sequence_id", false, true)
			if cell != "" && seq != "" {
				return fmt.Errorf("%s: Node.cell_id and sequence_id are mutually exclusive", path)
			}
		case "Cell":
			v, ok := fields["virtual_pairing"]
			if !ok || !(bytes.Equal(v, []byte("null")) || bytes.Equal(v, []byte("true")) || bytes.Equal(v, []byte("false"))) {
				return fmt.Errorf("%s/virtual_pairing must be explicitly boolean or null", path)
			}
			if receptors, ok := fields["receptors"]; ok && !bytes.Equal(receptors, []byte("null")) {
				var list []json.RawMessage
				if len(receptors) == 0 || receptors[0] != '[' || json.Unmarshal(receptors, &list) != nil {
					return fmt.Errorf("%s/receptors must be an array of identifiers or null", path)
				}
				if len(list) > 1000 {
					return fmt.Errorf("%s exceeds 1000 receptor links", path)
				}
				seen := map[string]bool{}
				for i, value := range list {
					id, err := objectID(map[string]json.RawMessage{"id": value}, "id", true, false)
					if err != nil {
						return err
					}
					if seen[id] {
						return fmt.Errorf("%s: repeated receptor link %q", path, id)
					}
					seen[id] = true
					o.Links = append(o.Links, ObjectLink{fmt.Sprintf("%s/receptors/%d", path, i), "Receptor", id, "unresolved"})
				}
			}
		case "Receptor":
			if err = checkReceptor(fields); err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
		}
		out.Objects = append(out.Objects, o)
		return nil
	}
	for _, kind := range []string{"Clone", "Cell", "Receptor", "Node"} {
		raw, ok := top[kind]
		if !ok {
			continue
		}
		var list []json.RawMessage
		if len(raw) == 0 || raw[0] != '[' || json.Unmarshal(raw, &list) != nil {
			return DataFile{}, fmt.Errorf("%s must be an array", kind)
		}
		for i, raw := range list {
			if err = add(kind, fmt.Sprintf("/%s/%d", kind, i), raw); err != nil {
				return DataFile{}, err
			}
		}
	}
	if err = ResolveObjectLinks(ctx, out.Objects, out.Repertoires); err != nil {
		return DataFile{}, err
	}
	return out, nil
}

func objectID(fields map[string]json.RawMessage, key string, required, nullable bool) (string, error) {
	raw, ok := fields[key]
	if !ok && !required {
		return "", nil
	}
	if ok && nullable && bytes.Equal(raw, []byte("null")) {
		return "", nil
	}
	var id string
	if !ok || json.Unmarshal(raw, &id) != nil || id == "" || strings.TrimSpace(id) != id || len(id) > 4096 || strings.ContainsFunc(id, unicode.IsControl) {
		return "", fmt.Errorf("%s must be a nonempty identifier of at most 4096 bytes without surrounding whitespace/control characters%s", key, map[bool]string{true: " (or explicit null)", false: ""}[nullable])
	}
	return id, nil
}
func checkReceptor(fields map[string]json.RawMessage) error {
	get := func(key string) string { var s string; _ = json.Unmarshal(fields[key], &s); return s }
	a, b := get("receptor_variable_domain_1_aa"), get("receptor_variable_domain_2_aa")
	for _, s := range []string{a, b} {
		if s == "" || strings.Trim(s, "ACDEFGHIKLMNPQRSTVWYXacdefghiklmnpqrstvwyx") != "" {
			return fmt.Errorf("variable domains must be nonempty amino-acid strings (20 standard residues or X); completeness is not inferred")
		}
	}
	l1, l2, t := get("receptor_variable_domain_1_locus"), get("receptor_variable_domain_2_locus"), get("receptor_type")
	if !((t == "Ig" && l1 == "IGH" && (l2 == "IGK" || l2 == "IGL")) || (t == "TCR" && ((l1 == "TRB" && l2 == "TRA") || (l1 == "TRD" && l2 == "TRG")))) {
		return fmt.Errorf("unsupported receptor_type/ordered variable-domain locus pairing")
	}
	hash := sha256.Sum256([]byte(strings.ToUpper(a) + strings.ToUpper(b)))
	if get("receptor_hash") != hex.EncodeToString(hash[:]) {
		return fmt.Errorf("receptor_hash does not match AIRR SHA-256 of the ordered uppercase variable domains")
	}
	return nil
}

// Study-local IDs are unique per kind within one opening (a conservative scope).
// No sequence/barcode matching, external lookup, or sole-processing fallback.
func ResolveObjectLinks(ctx context.Context, objects []Object, reps []Repertoire) error {
	if len(objects) > 100000 {
		return fmt.Errorf("combined AIRR objects exceed 100000")
	}
	index := map[string]int{}
	repIndex := map[string]Repertoire{}
	processing := map[string]bool{}
	for _, r := range reps {
		repIndex[r.RepertoireID] = r
	}
	for i, o := range objects {
		if err := ctx.Err(); err != nil {
			return err
		}
		key := o.Kind + "\x00" + o.ID
		if _, ok := index[key]; ok {
			return fmt.Errorf("duplicate %s identifier %q across imported objects", o.Kind, o.ID)
		}
		index[key] = i
	}
	// Validate processing IDs only in repertoires used by the new objects; legacy
	// repertoire-only inspection remains permissive rather than silently migrating.
	needed := map[string]bool{}
	for _, o := range objects {
		if o.RepertoireID != "" {
			needed[o.RepertoireID] = true
		}
	}
	for _, r := range reps {
		if !needed[r.RepertoireID] {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		var m map[string]json.RawMessage
		_ = json.Unmarshal(r.Raw, &m)
		raw, ok := m["data_processing"]
		if !ok || bytes.Equal(raw, []byte("null")) {
			continue
		}
		var list []map[string]json.RawMessage
		if len(raw) == 0 || raw[0] != '[' || json.Unmarshal(raw, &list) != nil {
			return fmt.Errorf("repertoire %q: data_processing must be an array", r.RepertoireID)
		}
		for _, p := range list {
			id, err := objectID(p, "data_processing_id", false, true)
			if err != nil {
				return err
			}
			if id != "" {
				key := r.RepertoireID + "\x00" + id
				if processing[key] {
					return fmt.Errorf("ambiguous data_processing_id %q in repertoire %q", id, r.RepertoireID)
				}
				processing[key] = true
			}
		}
	}
	total := 0
	for i := range objects {
		if err := ctx.Err(); err != nil {
			return err
		}
		o := &objects[i]
		total += len(o.Links)
		if total > 1000000 {
			return fmt.Errorf("combined AIRR links exceed 1000000")
		}
		for j := range o.Links {
			l := &o.Links[j]
			l.Status = "unresolved"
			switch l.Kind {
			case "Repertoire":
				if _, ok := repIndex[l.ID]; ok {
					l.Status = "loaded"
				}
			case "DataProcessing":
				if o.RepertoireID == "" {
					l.Status = "unscoped"
				} else if processing[o.RepertoireID+"\x00"+l.ID] {
					l.Status = "loaded"
				}
			case "Rearrangement", "RepertoireGroup":
				l.Status = "not-assessed"
			default:
				if n, ok := index[l.Kind+"\x00"+l.ID]; ok {
					target := objects[n]
					if o.RepertoireID != "" && target.RepertoireID != "" && o.RepertoireID != target.RepertoireID {
						return fmt.Errorf("%s: linked %s %q has a conflicting repertoire", o.Path, l.Kind, l.ID)
					}
					l.Status = "loaded"
				}
			}
		}
	}
	return nil
}
