package airr

// AIRR Repertoire metadata: AIRR Community Standards 2.0, Repertoire Schema,
// https://docs.airr-community.org/en/v2.0.0/datarep/metadata.html (linking/file format).
// ClonoLoom's import policy rejects ambiguous JSON/identifiers; this is not a full
// AIRR schema, ontology or MiAIRR completeness validator. Numbers never use float64.
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxMetadataDepth = 64
const maxMetadataValues = 1_000_000
const maxRepertoires = 100_000

// CheckExactJSON checks syntax, Unicode, duplicate keys and bounded structural
// work without converting numbers. Shared by native JSON importers; it does not
// validate any biological schema. Callers bound their input bytes separately.
func CheckExactJSON(ctx context.Context, raw []byte) error {
	return checkMetadataJSON(ctx, raw)
}

// RepertoireDocument keeps declared schema information separately from annotations.
// Raw repertoire objects retain their exact JSON bytes, including numeric tokens.
type RepertoireDocument struct {
	Repertoires   []Repertoire
	SchemaVersion string
}

func ReadRepertoireDocument(ctx context.Context, data []byte) (RepertoireDocument, error) {
	var out RepertoireDocument
	if len(data) > maxMetadataBytes {
		return out, errors.New("the metadata file is larger than 64 MB")
	}
	if err := checkMetadataJSON(ctx, data); err != nil {
		return out, err
	}
	data = bytes.TrimSpace(data)
	var objects []json.RawMessage
	switch data[0] {
	case '{':
		var top map[string]json.RawMessage
		if err := json.Unmarshal(data, &top); err != nil {
			return out, err
		}
		if raw, ok := top["Repertoire"]; ok {
			if len(raw) == 0 || raw[0] != '[' {
				return out, errors.New("Repertoire must be an array")
			}
			if err := json.Unmarshal(raw, &objects); err != nil {
				return out, err
			}
			if info, ok := top["Info"]; ok {
				var fields map[string]json.RawMessage
				if len(info) == 0 || info[0] != '{' {
					return out, errors.New("Info must be an object")
				}
				if err := json.Unmarshal(info, &fields); err != nil {
					return out, err
				}
				if version, ok := fields["version"]; ok {
					if len(version) == 0 || version[0] != '"' {
						return out, errors.New("Info.version must be a string")
					}
					if err := json.Unmarshal(version, &out.SchemaVersion); err != nil {
						return out, err
					}
				}
			}
		} else if _, ok := top["repertoire_id"]; ok {
			objects = []json.RawMessage{append(json.RawMessage(nil), data...)}
		}
	case '[':
		if err := json.Unmarshal(data, &objects); err != nil {
			return out, err
		}
	}
	if objects == nil {
		return out, errors.New("no AIRR Repertoire objects in the file")
	}
	if len(objects) > maxRepertoires {
		return out, fmt.Errorf("metadata exceeds %d repertoire records", maxRepertoires)
	}
	out.Repertoires = make([]Repertoire, 0, len(objects))
	seen := map[string]bool{}
	for i, raw := range objects {
		if err := ctx.Err(); err != nil {
			return RepertoireDocument{}, err
		}
		var m map[string]any
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		if err := dec.Decode(&m); err != nil || m == nil {
			return RepertoireDocument{}, fmt.Errorf("repertoire %d is not an object", i+1)
		}
		id, ok := m["repertoire_id"].(string)
		if !ok || id == "" || strings.TrimSpace(id) != id || len(id) > 4096 || strings.ContainsFunc(id, unicode.IsControl) {
			return RepertoireDocument{}, fmt.Errorf("repertoire %d: repertoire_id must be a nonempty string of at most 4096 bytes without surrounding whitespace or control characters", i+1)
		}
		if seen[id] {
			return RepertoireDocument{}, fmt.Errorf("duplicate repertoire_id %q (record %d); give each record a unique identifier", id, i+1)
		}
		seen[id] = true
		out.Repertoires = append(out.Repertoires, repertoireFromObject(m, raw))
	}
	return out, nil
}

// Token inspection rejects duplicate keys even when their spellings use escapes.
// Bound the walk before decoding nested structures. Check cancellation between values.
func checkMetadataJSON(ctx context.Context, raw []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !utf8.Valid(raw) {
		return errors.New("metadata must be valid UTF-8")
	}
	if !json.Valid(raw) {
		return errors.New("metadata must contain exactly one valid JSON value")
	}
	// encoding/json replaces unpaired UTF-16 surrogates instead of reporting them.
	// Refuse those escapes so two different source identifiers cannot collapse to U+FFFD.
	for i := 0; i < len(raw); i++ {
		if i%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		if raw[i] != '\\' {
			continue
		}
		i++
		if raw[i] != 'u' {
			continue
		}
		n, _ := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		i += 4
		if n >= 0xdc00 && n <= 0xdfff {
			return errors.New("metadata contains an unpaired Unicode surrogate")
		}
		if n >= 0xd800 && n <= 0xdbff {
			if i+6 >= len(raw) || string(raw[i+1:i+3]) != "\\u" {
				return errors.New("metadata contains an unpaired Unicode surrogate")
			}
			low, err := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
			if err != nil || low < 0xdc00 || low > 0xdfff {
				return errors.New("metadata contains an unpaired Unicode surrogate")
			}
			i += 6
		}
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	values := 0
	var walk func(int) error
	walk = func(depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if depth > maxMetadataDepth {
			return fmt.Errorf("metadata JSON nesting exceeds %d", maxMetadataDepth)
		}
		values++
		if values > maxMetadataValues {
			return fmt.Errorf("metadata exceeds %d JSON values", maxMetadataValues)
		}
		token, err := d.Token()
		if err != nil {
			return err
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok || seen[name] {
					return fmt.Errorf("duplicate or invalid metadata JSON key %q", name)
				}
				seen[name] = true
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for d.More() {
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
		default:
			return errors.New("invalid metadata delimiter")
		}
		_, err = d.Token()
		return err
	}
	if err := walk(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("metadata must contain exactly one JSON value")
	}
	return nil
}
