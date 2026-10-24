package hdf5

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "expression-hdf5", name+".h5"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func readTest(t *testing.T, b []byte) *Reader {
	t.Helper()
	f, err := Open(context.Background(), bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestHDF5BoundsChecksumsAndCycles(t *testing.T) {
	base := fixture(t, "checksum")
	modern := fixture(t, "v108")
	for _, n := range []int{0, 7, 8, 12, 16, 40, len(base) - 1} {
		if _, err := Open(context.Background(), bytes.NewReader(base[:n]), int64(n)); err == nil {
			t.Fatalf("truncated size %d accepted", n)
		}
	}
	if _, err := Open(context.Background(), bytes.NewReader(nil), MaxFile+1); err == nil {
		t.Fatal("file bound ignored")
	}
	for _, test := range []struct {
		name       string
		source     []byte
		mutate     func([]byte, *Reader)
		path, want string
	}{
		{"superblock-checksum", modern, func(b []byte, f *Reader) { b[36] ^= 1 }, "", "checksum"},
		{"object-checksum", modern, func(b []byte, f *Reader) { b[f.root+6] ^= 1 }, "/matrix/shape", "checksum"},
		{"group-cycle", base, func(b []byte, f *Reader) {
			at := uint64(bytes.Index(b, []byte("TREE")))
			b[at+5] = 1
			le.PutUint64(b[at+32:], at)
		}, "/matrix/shape", "cycle"},
		{"shared-message", base, func(b []byte, f *Reader) { b[f.root+20] |= 2 }, "/matrix/shape", "shared"},
		{"chunk-corruption", base, func(b []byte, f *Reader) {
			at, err := f.resolve("/matrix/data")
			if err != nil {
				t.Fatal(err)
			}
			ms, err := f.object(at)
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range ms {
				if m.kind == 8 {
					tree := le.Uint64(m.data[3:])
					chunk := le.Uint64(b[tree+48:])
					b[chunk] ^= 1
					return
				}
			}
			t.Fatal("no chunk layout")
		}, "/matrix/data", "checksum"},
		{"chunk-cycle", base, func(b []byte, f *Reader) {
			at, _ := f.resolve("/matrix/data")
			ms, _ := f.object(at)
			for _, m := range ms {
				if m.kind == 8 {
					tree := le.Uint64(m.data[3:])
					b[tree+5] = 1
					le.PutUint64(b[tree+48:], tree)
					return
				}
			}
		}, "/matrix/data", "cycle"},
		{"chunk-outside-file", base, func(b []byte, f *Reader) {
			at, _ := f.resolve("/matrix/data")
			ms, _ := f.object(at)
			for _, m := range ms {
				if m.kind == 8 {
					tree := le.Uint64(m.data[3:])
					le.PutUint64(b[tree+48:], uint64(len(b)))
					return
				}
			}
		}, "/matrix/data", "outside file"},
	} {
		t.Run(test.name, func(t *testing.T) {
			b := bytes.Clone(test.source)
			f := readTest(t, b)
			test.mutate(b, f)
			f, err := Open(context.Background(), bytes.NewReader(b), int64(len(b)))
			if err == nil {
				_, err = f.Integers(test.path, 100)
			}
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("want %q, got %v", test.want, err)
			}
		})
	}
}

func TestHDF5ResourceBudgetsAndFilters(t *testing.T) {
	b := fixture(t, "checksum")
	for _, prepare := range []func(*Reader){func(f *Reader) { f.metadata = maxMetadata }, func(f *Reader) { f.readBytes = 2 << 30 }, func(f *Reader) { f.nodes = 1000000 }, func(f *Reader) { f.decoded = 512 << 20 }} {
		f := readTest(t, b)
		prepare(f)
		if _, err := f.Integers("/matrix/data", 100); err == nil {
			t.Fatal("budget ignored")
		}
	}
	f := readTest(t, b)
	if _, err := f.Integers("/matrix/data", 5); err == nil {
		t.Fatal("element limit ignored")
	}
	if _, err := f.unfilter([]byte{1, 2}, nil, 1, 2, 1); err == nil {
		t.Fatal("invalid filter mask accepted")
	}
	if _, err := f.unfilter([]byte{1, 2}, []filter{{id: 3}}, 1, 2, 1); err != nil {
		t.Fatal("skipped filter not honored", err)
	}
	if _, err := f.unfilter([]byte{1, 2}, nil, 0, 3, 1); err == nil {
		t.Fatal("wrong size accepted")
	}
	if _, err := typeInfo([]byte{0x10, 0, 0, 0, 4, 0, 0, 0, 1, 0, 32, 0}); err == nil {
		t.Fatal("offset integer accepted")
	}
}

func TestFletcher32IndependentH5pyChunks(t *testing.T) {
	// Raw chunks written/read by h5py 3.15.1 with only Fletcher32 enabled. One's-complement
	// arithmetic preserves 0xffff; reducing modulo 65535 would wrongly turn it into zero.
	for _, v := range []struct {
		data     []byte
		checksum uint32
	}{
		{[]byte{0, 0, 0, 0}, 0}, {[]byte{255, 255, 255, 255}, 0xffffffff},
		{[]byte{1, 2, 3}, 0x05040402}, {[]byte{255, 255, 255}, 0xff00ff00},
	} {
		if got := fletcher(v.data); got != v.checksum {
			t.Fatalf("%x: got %x, want %x", v.data, got, v.checksum)
		}
	}
}
