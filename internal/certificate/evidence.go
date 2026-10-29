package certificate

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
)

// Evidence opens only inventoried members after integrity verification. Read checks
// the pinned bytes again so a concurrently changed input cannot replace replay inputs.
type Evidence struct {
	record Record
	zip    *zip.Reader
}

func OpenEvidence(ctx context.Context, in io.ReaderAt, size int64, expected string) (*Evidence, Report, error) {
	report := Verify(ctx, in, size, expected)
	if report.Verdict != "integrity-match" {
		return nil, report, errors.New("certificate integrity check failed")
	}
	z, err := zip.NewReader(in, size)
	if err != nil {
		return nil, report, err
	}
	var raw []byte
	for _, f := range z.File {
		if f.Name == "certificate.json" {
			r, e := f.Open()
			if e != nil {
				return nil, report, e
			}
			raw, e = io.ReadAll(&contextReader{ctx, io.LimitReader(r, MaxRecordBytes+1)})
			r.Close()
			if e != nil {
				return nil, report, e
			}
		}
	}
	var record Record
	if err = json.Unmarshal(raw, &record); err != nil {
		return nil, report, err
	}
	canonical, _ := json.Marshal(record)
	if !bytes.Equal(raw, canonical) || record.Fingerprint != report.Fingerprint || record.fingerprint() != report.Fingerprint {
		return nil, report, errors.New("certificate changed after integrity checking")
	}
	return &Evidence{record, z}, report, nil
}
func (e *Evidence) Read(ctx context.Context, name string, limit int64) ([]byte, error) {
	for _, item := range e.record.Files {
		if item.Path != name {
			continue
		}
		if item.Size > limit {
			return nil, errors.New("replay input exceeds limit")
		}
		for _, f := range e.zip.File {
			if f.Name != name {
				continue
			}
			r, err := f.Open()
			if err != nil {
				return nil, err
			}
			raw, err := io.ReadAll(&contextReader{ctx, io.LimitReader(r, item.Size+1)})
			r.Close()
			if err != nil {
				return nil, err
			}
			if int64(len(raw)) != item.Size || sum(raw) != item.SHA256 {
				return nil, errors.New("replay input changed after integrity checking")
			}
			return raw, nil
		}
	}
	return nil, errors.New("required replay evidence is missing")
}
