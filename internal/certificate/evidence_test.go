package certificate

import (
	"bytes"
	"context"
	"io"
	"testing"
)

func TestEvidencePinsBytesAndBounds(t *testing.T) {
	raw, record := fixture(t)
	e, report, err := OpenEvidence(context.Background(), bytes.NewReader(raw), int64(len(raw)), record.Fingerprint)
	if err != nil || report.Verdict != "integrity-match" {
		t.Fatal(report, err)
	}
	result, err := e.Read(context.Background(), "artifact/result.json", MaxFileBytes)
	if err != nil || !bytes.Contains(result, []byte("1.2345678901234567")) {
		t.Fatal(string(result), err)
	}
	if _, err = e.Read(context.Background(), "artifact/result.json", 1); err == nil {
		t.Fatal("ignored caller bound")
	}
	if _, err = e.Read(context.Background(), "missing", MaxFileBytes); err == nil {
		t.Fatal("missing input accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = e.Read(ctx, "artifact/result.json", MaxFileBytes); err == nil {
		t.Fatal("ignored cancellation")
	}
	// The ZIP reader holds offsets into the original ReaderAt, not a trusted copy.
	// Change the compressed member after verification, before it is used for replay.
	for _, f := range e.zip.File {
		if f.Name == "artifact/result.json" {
			offset, err := f.DataOffset()
			if err != nil {
				t.Fatal(err)
			}
			raw[offset] ^= 0xff
		}
	}
	if _, err = e.Read(context.Background(), "artifact/result.json", MaxFileBytes); err == nil {
		t.Fatal("accepted a replaced replay input")
	}
}
func TestEvidenceRejectsChangedContainerAfterVerification(t *testing.T) {
	raw, _ := fixture(t)
	r := &changingReader{raw: raw}
	_, _, err := OpenEvidence(context.Background(), r, int64(len(raw)), "")
	if err == nil {
		t.Fatal("accepted a changed container")
	}
}

// Verify begins with a ZIP trailer read; the second trailer read belongs to the
// reopened Evidence reader. A failed read there must never yield replay evidence.
type changingReader struct {
	raw      []byte
	trailers int
}

func (r *changingReader) ReadAt(p []byte, off int64) (int, error) {
	if off+int64(len(p)) == int64(len(r.raw)) {
		r.trailers++
		if r.trailers > 1 {
			return 0, io.ErrUnexpectedEOF
		}
	}
	return bytes.NewReader(r.raw).ReadAt(p, off)
}
