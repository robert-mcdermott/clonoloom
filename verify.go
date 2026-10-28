package main

// CLI shape and explicit verdicts follow CytoWeave 0.8.1 / MaveScape verify.go.
// Unlike their browser-based implementations, this checks bytes directly in Go.
// Wave 8 slice 1 requires --integrity: it must never imply scientific replay.
import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"clonoloom/internal/certificate"
)

func runVerify(args []string, stdout, stderr io.Writer) int {
	f := flag.NewFlagSet("clonoloom verify", flag.ContinueOnError)
	f.SetOutput(stderr)
	integrity := f.Bool("integrity", false, "check recorded bytes only; no scientific recomputation")
	expect := f.String("expect", "", "an independently recorded certificate SHA-256 fingerprint")
	report := f.String("report", "", "write the verification JSON to a new file (never overwrite)")
	timeout := f.Duration("timeout", 5*time.Minute, "maximum verification time")
	f.Usage = func() {
		fmt.Fprintln(stderr, "Usage: clonoloom verify --integrity [--expect SHA256] [--report NEW.json] FILE.certificate.clz\nExit status: 0 integrity match; 1 contents differ or archive is invalid; 2 usage, input, output or cancellation error.\nNo scientific recomputation, source-file or parent verification is performed.")
		f.PrintDefaults()
	}
	paths, err := parseArgs(f, args)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		return 2
	}
	bad := func(err error) int { fmt.Fprintln(stderr, "clonoloom verify:", err); return 2 }
	if !*integrity || len(paths) != 1 || *timeout <= 0 {
		return bad(errors.New("give --integrity, one certificate and a positive timeout; scientific replay is not available in this slice"))
	}
	if *expect != "" {
		raw, e := hex.DecodeString(*expect)
		if e != nil || len(raw) != 32 || strings.ToLower(*expect) != *expect {
			return bad(errors.New("--expect must be 64 lowercase hexadecimal characters"))
		}
	}
	file, err := os.Open(paths[0])
	if err != nil {
		return bad(err)
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return bad(err)
	}
	if !stat.Mode().IsRegular() {
		return bad(errors.New("input must be a regular certificate file"))
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	answer := certificate.Verify(ctx, file, stat.Size(), *expect)
	if err = ctx.Err(); err != nil {
		return bad(err)
	}
	if *report != "" {
		out, e := os.OpenFile(*report, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return bad(e)
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		e = enc.Encode(answer)
		closed := out.Close()
		if e == nil {
			e = closed
		}
		if e != nil {
			os.Remove(*report)
			return bad(e)
		}
	}
	fmt.Fprintf(stdout, "%s: %d files, %d bytes checked\nFingerprint: %s\n%s\n", answer.Verdict, answer.FilesChecked, answer.BytesChecked, answer.Fingerprint, answer.Limits)
	for _, finding := range answer.Findings {
		fmt.Fprintln(stdout, finding)
	}
	if answer.Verdict != "integrity-match" {
		return 1
	}
	return 0
}
