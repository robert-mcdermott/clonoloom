package reads

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// Capture and hash the same bounded source bytes once. A later source mutation
// cannot change the fingerprint of the metadata held by the opened dataset.
func readMetadata(ctx context.Context, path string) ([]byte, MetadataFile, error) {
	var file MetadataFile
	f, err := os.Open(path)
	if err != nil {
		return nil, file, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, file, err
	}
	if !st.Mode().IsRegular() {
		return nil, file, errors.New("metadata must be a regular file")
	}
	raw, err := io.ReadAll(io.LimitReader(metadataReader{ctx, f}, maxMetadataBytes+1))
	if err != nil {
		return nil, file, err
	}
	if len(raw) > maxMetadataBytes {
		return nil, file, errors.New("the metadata source is larger than 64 MB")
	}
	end, err := f.Stat()
	if err != nil {
		return nil, file, err
	}
	if end.Size() != st.Size() || !end.ModTime().Equal(st.ModTime()) || int64(len(raw)) != st.Size() {
		return nil, file, errors.New("metadata changed while reading; reopen the dataset")
	}
	sum := sha256.Sum256(raw)
	file = MetadataFile{Path: path, Size: int64(len(raw)), SHA256: hex.EncodeToString(sum[:]), Modified: st.ModTime().UTC().Format(time.RFC3339)}
	data := raw
	if len(raw) >= 2 && raw[0] == 0x1f && raw[1] == 0x8b {
		gz, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, file, err
		}
		defer gz.Close()
		data, err = io.ReadAll(io.LimitReader(metadataReader{ctx, gz}, maxMetadataBytes+1))
		if err != nil {
			return nil, file, fmt.Errorf("metadata gzip: %w", err)
		}
		if len(data) > maxMetadataBytes {
			return nil, file, errors.New("the decompressed metadata is larger than 64 MB")
		}
	}
	return data, file, ctx.Err()
}

type metadataReader struct {
	ctx context.Context
	r   io.Reader
}

func (r metadataReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}
