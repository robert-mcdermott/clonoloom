package rep

import (
	"errors"
	"math"
)

// Strings is an append-only store of strings in one byte arena, each named by a code: 1, 2, 3…
// in the order added. Code 0 is the empty string, which stands for a missing value. A store that
// interns (NewStrings(true)) gives a string already present its first code, so a column of
// repeated values (gene calls, barcodes, junctions) costs four bytes a row and each distinct
// string once; Freeze drops the index when no more strings will be added.
type Strings struct {
	data   []byte
	ends   []uint32 // ends[i] is where the string with code i+1 ends
	index  map[string]uint32
	intern bool
}

// ErrTooLarge: a store holds at most 4 GiB of text and 2³²−1 strings.
var ErrTooLarge = errors.New("more text than one table holds (4 GiB)")

func NewStrings(intern bool) *Strings {
	s := &Strings{intern: intern}
	if intern {
		s.index = map[string]uint32{}
	}
	return s
}

// Add stores v and answers its code (0 for "").
func (s *Strings) Add(v string) (uint32, error) {
	if v == "" {
		return 0, nil
	}
	if s.intern {
		if code, ok := s.index[v]; ok {
			return code, nil
		}
		if s.index == nil {
			return 0, errors.New("the string store was frozen")
		}
	}
	if uint64(len(s.data))+uint64(len(v)) > math.MaxUint32 || len(s.ends) >= math.MaxUint32-1 {
		return 0, ErrTooLarge
	}
	s.data = append(s.data, v...)
	s.ends = append(s.ends, uint32(len(s.data)))
	code := uint32(len(s.ends))
	if s.intern {
		s.index[string(s.data[len(s.data)-len(v):])] = code
	}
	return code, nil
}

// Get is the string with a code ("" for 0 or a code not given).
func (s *Strings) Get(code uint32) string {
	if code == 0 || int(code) > len(s.ends) {
		return ""
	}
	start := uint32(0)
	if code > 1 {
		start = s.ends[code-2]
	}
	return string(s.data[start:s.ends[code-1]])
}

// Bytes is the string with a code as a slice of the arena, which the caller must not change.
func (s *Strings) Bytes(code uint32) []byte {
	if code == 0 || int(code) > len(s.ends) {
		return nil
	}
	start := uint32(0)
	if code > 1 {
		start = s.ends[code-2]
	}
	return s.data[start:s.ends[code-1]]
}

// Len is the number of strings stored (codes 1…Len).
func (s *Strings) Len() int { return len(s.ends) }

// Lookup answers a string's code in an interning store that is not frozen.
func (s *Strings) Lookup(v string) (uint32, bool) {
	if v == "" {
		return 0, true
	}
	code, ok := s.index[v]
	return code, ok
}

// Freeze drops the index: the store's strings stay readable, and no more can be added.
func (s *Strings) Freeze() {
	s.index = nil
	s.data = s.data[:len(s.data):len(s.data)]
	s.ends = s.ends[:len(s.ends):len(s.ends)]
}

// Size is the memory the store's text and codes take, in bytes (without the index).
func (s *Strings) Size() int64 {
	return int64(cap(s.data)) + 4*int64(cap(s.ends))
}
