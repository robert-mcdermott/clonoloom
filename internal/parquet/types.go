// Package parquet writes bounded flat Parquet files; it is not a general reader.
// Types adapted from SituCraft v0.9.0 (Apache-2.0), internal/parquet/{metadata,parquet}.go.
package parquet

import "fmt"

type Type int32

const (
	Boolean Type = iota
	Int32
	Int64
	Int96
	Float
	Double
	ByteArray
	FixedLenByteArray
)

func (t Type) String() string {
	names := []string{"BOOLEAN", "INT32", "INT64", "INT96", "FLOAT", "DOUBLE", "BYTE_ARRAY", "FIXED_LEN_BYTE_ARRAY"}
	if t >= 0 && int(t) < len(names) {
		return names[t]
	}
	return fmt.Sprintf("type %d", t)
}

type Values struct {
	Valid   []bool
	Bool    []bool
	Int32   []int32
	Int64   []int64
	Float32 []float32
	Float64 []float64
	Bytes   [][]byte
}

const (
	encPlain = 0
	encRLE   = 3
	pageData = 0
	Gzip     = 2
)
const (
	tStop   = 0
	tTrue   = 1
	tFalse  = 2
	tByte   = 3
	tI32    = 5
	tI64    = 6
	tBinary = 8
	tList   = 9
	tStruct = 12
)
