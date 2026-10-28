package main

// Automation policy is shared by all remote transports. Capabilities describe conservative
// upper bounds, not promises that a particular invocation exercises every effect.
import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"reflect"
	"strings"
	"time"
)

const actionCatalogVersion = 1
const automationPolicyVersion = 1

type automationAudit struct {
	Time       string `json:"time"`
	Client     string `json:"client"`
	Action     string `json:"action"`
	State      string `json:"state"`
	ArgsSHA256 string `json:"argsSha256"`
}

func parseAutomationAllow(s string) (map[string]bool, error) {
	out := map[string]bool{"read": true}
	if s == "" {
		return out, nil
	}
	for _, effect := range strings.Split(s, ",") {
		switch effect {
		case "edits", "compute", "files", "network":
		default:
			return nil, fmt.Errorf("--allow: unknown capability %q (edits,compute,files,network)", effect)
		}
		if out[effect] {
			return nil, fmt.Errorf("--allow: repeated capability %q", effect)
		}
		out[effect] = true
	}
	return out, nil
}

func (h *remoteHub) record(event remoteEvent, state string) {
	row := automationAudit{Time: time.Now().UTC().Format(time.RFC3339Nano), Client: event.Client, Action: event.Action, State: state, ArgsSHA256: fmt.Sprintf("%x", sha256.Sum256(event.Args))}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.audit = append(h.audit, row)
	if len(h.audit) > 500 {
		h.audit = append([]automationAudit(nil), h.audit[len(h.audit)-500:]...)
	}
	// Audit delivery is advisory; GET /audit is the bounded authoritative session inventory.
	for _, client := range h.clients {
		select {
		case client.events <- remoteEvent{Audit: &row}:
		default:
		}
	}
}

func strictJSON(data []byte, target any) error {
	if err := uniqueJSONKeys(json.NewDecoder(bytes.NewReader(data)), 0); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	dec.UseNumber()
	if err := dec.Decode(target); err != nil {
		return err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("expected one JSON value")
	}
	return nil
}

// The catalog's declared shapes are enforced before dispatch. Scientific nested options
// still have their existing method-specific validators; unknown top-level arguments fail.
func validateActionArgs(raw json.RawMessage, schema map[string]any) error {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	var value any
	if err := strictJSON(raw, &value); err != nil {
		return err
	}
	if err := automationNumbers(value, 0); err != nil {
		return err
	}
	return validateActionValue(value, schema, 0)
}
func validateActionValue(value any, s map[string]any, depth int) error {
	if depth > 32 {
		return fmt.Errorf("arguments are too deeply nested")
	}
	switch s["type"] {
	case "object":
		v, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("expected an object")
		}
		props, _ := s["properties"].(map[string]any)
		required, _ := s["required"].([]string)
		for _, k := range required {
			if _, ok := v[k]; !ok {
				return fmt.Errorf("missing argument %s", k)
			}
		}
		for k, x := range v {
			child, known := props[k]
			if !known {
				if s["additionalProperties"] == false {
					return fmt.Errorf("unknown argument %s", k)
				}
				continue
			}
			if err := validateActionValue(x, child.(map[string]any), depth+1); err != nil {
				return fmt.Errorf("%s: %w", k, err)
			}
		}
	case "array":
		v, ok := value.([]any)
		if !ok {
			return fmt.Errorf("expected an array")
		}
		if n, ok := s["minItems"].(int); ok && len(v) < n {
			return fmt.Errorf("too few items")
		}
		if n, ok := s["maxItems"].(int); ok && len(v) > n {
			return fmt.Errorf("too many items")
		}
		for i, x := range v {
			if child, ok := s["items"].(map[string]any); ok {
				if err := validateActionValue(x, child, depth+1); err != nil {
					return err
				}
			}
			if s["uniqueItems"] == true {
				for _, prior := range v[:i] {
					if reflect.DeepEqual(x, prior) {
						return fmt.Errorf("duplicate item")
					}
				}
			}
		}
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("expected a string")
		}
	case "number", "integer":
		if _, ok := value.(json.Number); !ok {
			return fmt.Errorf("expected a number")
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected a boolean")
		}
	}
	return nil
}

// Browser dispatch must never silently round a large integer argument. Fractional
// scientific parameters retain the ordinary native IEEE-754 method contract.
func automationNumbers(value any, depth int) error {
	if depth > 32 {
		return fmt.Errorf("arguments exceed 32 levels")
	}
	switch v := value.(type) {
	case json.Number:
		n, err := v.Float64()
		if err != nil || math.IsInf(n, 0) || (math.Trunc(n) == n && math.Abs(n) > 9007199254740991) {
			return fmt.Errorf("numeric argument cannot be represented safely by the window")
		}
	case map[string]any:
		for _, x := range v {
			if err := automationNumbers(x, depth+1); err != nil {
				return err
			}
		}
	case []any:
		for _, x := range v {
			if err := automationNumbers(x, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

// Reject duplicate object keys: a review must not depend on which JSON reader wins.
func uniqueJSONKeys(dec *json.Decoder, depth int) error {
	if depth > 64 {
		return fmt.Errorf("JSON nesting exceeds 64 levels")
	}
	token, err := dec.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		for dec.More() {
			key, err := dec.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return fmt.Errorf("duplicate or invalid JSON field %q", key)
			}
			seen[name] = true
			if err := uniqueJSONKeys(dec, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for dec.More() {
			if err := uniqueJSONKeys(dec, depth+1); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter")
	}
	_, err = dec.Token()
	return err
}
