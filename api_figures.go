package main

// Figures as PDF (clonoloom-spec/requirements.md, O1): the page draws a figure (web/lib/plot.js,
// figure) and renders it as a PNG at 300 dots per inch; this wraps the PNG in a one-page PDF of
// the figure's size, written by hand (the format's objects, an image compressed with zlib). The
// PDF holds the figure as an image; SVG is the vector form.

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// maxFigurePixels bounds a figure's image (a 50 × 50 cm figure at 300 dpi is about 35 million).
const maxFigurePixels = 60_000_000

func (a *app) registerFigures(mux *http.ServeMux) {
	// POST /api/figure/pdf?width=&height=&title= with a PNG: the PDF, width × height in CSS pixels
	// (1/96 inch), the image scaled to fill it.
	mux.HandleFunc("POST /api/figure/pdf", func(w http.ResponseWriter, r *http.Request) {
		width, err1 := strconv.ParseFloat(r.URL.Query().Get("width"), 64)
		height, err2 := strconv.ParseFloat(r.URL.Query().Get("height"), 64)
		if err1 != nil || err2 != nil || width <= 0 || height <= 0 || width > 20000 || height > 20000 {
			writeError(w, http.StatusBadRequest, "Send the figure's width and height in pixels.")
			return
		}
		raw, err := io.ReadAll(io.LimitReader(r.Body, 200<<20))
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if c, err := png.DecodeConfig(bytes.NewReader(raw)); err != nil || c.Width*c.Height > maxFigurePixels {
			writeError(w, http.StatusBadRequest, "Send the figure as a PNG of at most 60 million pixels.")
			return
		}
		img, err := png.Decode(bytes.NewReader(raw))
		if err != nil {
			writeError(w, http.StatusBadRequest, "The figure is not a PNG ClonoLoom reads.")
			return
		}
		pdf, err := figurePDF(img, width*0.75, height*0.75, r.URL.Query().Get("title"), time.Now())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Write(pdf)
	})
}

// figurePDF writes a one-page PDF of the image, the page width × height points, the image on white
// (its transparency flattened).
func figurePDF(img image.Image, width, height float64, title string, made time.Time) ([]byte, error) {
	b := img.Bounds()
	pixels := make([]byte, 0, b.Dx()*b.Dy()*3)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, al := img.At(x, y).RGBA()
			// Over white: c + (1 − α) for each 16-bit channel (already premultiplied).
			white := 0xffff - al
			pixels = append(pixels, byte((r+white)>>8), byte((g+white)>>8), byte((bl+white)>>8))
		}
	}
	var z bytes.Buffer
	zw, err := zlib.NewWriterLevel(&z, zlib.BestCompression)
	if err != nil {
		return nil, err
	}
	zw.Write(pixels)
	if err := zw.Close(); err != nil {
		return nil, err
	}
	content := fmt.Sprintf("q %.2f 0 0 %.2f 0 0 cm /Im0 Do Q", width, height)
	var out bytes.Buffer
	offsets := []int{}
	object := func(body string, stream []byte) {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n%s", len(offsets), body)
		if stream != nil {
			out.WriteString("\nstream\n")
			out.Write(stream)
			out.WriteString("\nendstream")
		}
		out.WriteString("\nendobj\n")
	}
	out.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	object("<< /Type /Catalog /Pages 2 0 R >>", nil)
	object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>", nil)
	object(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.2f %.2f] /Resources << /XObject << /Im0 4 0 R >> >> /Contents 5 0 R >>", width, height), nil)
	object(fmt.Sprintf("<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /FlateDecode /Length %d >>", b.Dx(), b.Dy(), z.Len()), z.Bytes())
	object(fmt.Sprintf("<< /Length %d >>", len(content)), []byte(content))
	object(fmt.Sprintf("<< /Title %s /Producer (ClonoLoom %s) /CreationDate (D:%s) >>", pdfString(title), version, made.UTC().Format("20060102150405Z")), nil)
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offsets)+1)
	for _, o := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R /Info %d 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets)+1, len(offsets), xref)
	return out.Bytes(), nil
}

// pdfString is a PDF literal string: ASCII as it is (parentheses and backslashes escaped), anything
// else as UTF-16 with its byte-order mark, in hexadecimal.
func pdfString(s string) string {
	ascii := true
	for _, r := range s {
		if r > 126 || r < 32 {
			ascii = false
		}
	}
	if ascii {
		return "(" + strings.NewReplacer(`\`, `\\`, "(", `\(`, ")", `\)`).Replace(s) + ")"
	}
	var hex strings.Builder
	hex.WriteString("<FEFF")
	for _, r := range s {
		if r > 0xffff {
			r -= 0x10000
			fmt.Fprintf(&hex, "%04X%04X", 0xd800+(r>>10), 0xdc00+(r&0x3ff))
			continue
		}
		fmt.Fprintf(&hex, "%04X", r)
	}
	hex.WriteString(">")
	return hex.String()
}
