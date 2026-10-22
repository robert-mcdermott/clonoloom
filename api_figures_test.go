package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"regexp"
	"strconv"
	"testing"
	"time"
)

// A figure's PNG becomes a one-page PDF of its size, its objects where the cross-reference table
// says they are, its transparency on white.
func TestFigurePDF(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 30, 20))
	for y := range 20 {
		for x := range 30 {
			img.Set(x, y, color.NRGBA{R: 91, G: 76, B: 230, A: uint8(255 * (x % 2))})
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	_, handler := testApp(t)
	rec := request(t, handler, http.MethodPost, "/api/figure/pdf?width=400&height=300&title=Shannon%20diversity%20%E2%80%94%20S1", bytes.NewReader(buf.Bytes()), map[string]string{"Content-Type": "image/png"})
	pdf := rec.Body.Bytes()
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/pdf" || !bytes.HasPrefix(pdf, []byte("%PDF-1.4")) || !bytes.HasSuffix(pdf, []byte("%%EOF\n")) {
		t.Fatalf("%d %q", rec.Code, pdf[:min(len(pdf), 40)])
	}
	// 400 × 300 pixels is 300 × 225 points.
	if !bytes.Contains(pdf, []byte("/MediaBox [0 0 300.00 225.00]")) || !bytes.Contains(pdf, []byte("/Width 30 /Height 20")) || !bytes.Contains(pdf, []byte("/Title <FEFF")) {
		t.Errorf("%s", pdf[:min(len(pdf), 600)])
	}
	// Every object at the offset the table gives.
	xref := regexp.MustCompile(`startxref\n(\d+)`).FindSubmatch(pdf)
	at, _ := strconv.Atoi(string(xref[1]))
	entries := regexp.MustCompile(`(\d{10}) 00000 n `).FindAllSubmatch(pdf[at:], -1)
	if len(entries) != 6 {
		t.Fatalf("%d objects", len(entries))
	}
	for i, e := range entries {
		off, _ := strconv.Atoi(string(e[1]))
		if want := strconv.Itoa(i+1) + " 0 obj"; !bytes.HasPrefix(pdf[off:], []byte(want)) {
			t.Errorf("object %d at %d: %q", i+1, off, pdf[off:off+10])
		}
	}
	// Not a PNG, or no size: refused.
	for _, bad := range []string{"/api/figure/pdf?width=400&height=300", "/api/figure/pdf?width=0&height=300"} {
		body := []byte("not a png")
		if bad == "/api/figure/pdf?width=0&height=300" {
			body = buf.Bytes()
		}
		if rec := request(t, handler, http.MethodPost, bad, bytes.NewReader(body), nil); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", bad, rec.Code)
		}
	}
	// Transparent pixels on white.
	out, _ := figurePDF(img, 10, 10, "", time.Unix(0, 0))
	if !bytes.Contains(out, []byte("(ClonoLoom ")) {
		t.Error("no producer")
	}
}

func TestPDFString(t *testing.T) {
	for in, want := range map[string]string{"Plain (a)": `(Plain \(a\))`, "Sørensen": "<FEFF005300F800720065006E00730065006E>", "": "()"} {
		if got := pdfString(in); got != want {
			t.Errorf("%q: %s, want %s", in, got, want)
		}
	}
}
