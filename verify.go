package main

// CLI shape and explicit verdicts follow CytoWeave 0.8.1 / MaveScape verify.go.
// Unlike their browser-based implementations, this checks bytes directly in Go.
// Explicit modes never substitute an integrity match for scientific replay.
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
	replay := f.Bool("replay", false, "recompute supported cohort results from captured evidence, exactly")
	expect := f.String("expect", "", "an independently recorded certificate SHA-256 fingerprint")
	report := f.String("report", "", "write the verification JSON to a new file (never overwrite)")
	timeout := f.Duration("timeout", 5*time.Minute, "maximum verification time")
	f.Usage = func() {
		fmt.Fprintln(stderr, "Usage: clonoloom verify (--integrity | --replay) [--expect SHA256] [--report NEW.json] FILE.certificate.clz\nExit status: 0 requested check matches; 1 contents differ or archive is invalid; 2 usage, input, output, cancellation or unavailable replay.\nReplay checks captured cohort evidence; original sources and upstream diversity are not recomputed.")
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
	if *integrity == *replay || len(paths) != 1 || *timeout <= 0 {
		return bad(errors.New("give exactly one of --integrity or --replay, one certificate and a positive timeout"))
	}
	if *expect != "" {
		raw, e := hex.DecodeString(*expect)
		if e != nil || len(raw) != 32 || strings.ToLower(*expect) != *expect {
			return bad(errors.New("--expect must be 64 lowercase hexadecimal characters"))
		}
	}
	// Refuse an existing destination before expensive replay. O_EXCL below still
	// protects against a file created between this check and the final write.
	if *report != "" {
		if _, e := os.Lstat(*report); e == nil {
			return bad(errors.New("report path already exists"))
		} else if !errors.Is(e, os.ErrNotExist) {
			return bad(e)
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
	var answer any
	var verdict, fingerprint, limits string
	var findings []string
	if *replay {
		r := replayCertificate(ctx, file, stat.Size(), *expect)
		answer, verdict, fingerprint, limits, findings = r, r.Verdict, r.Integrity.Fingerprint, r.Limits, r.Findings
	} else {
		r := certificate.Verify(ctx, file, stat.Size(), *expect)
		answer, verdict, fingerprint, limits, findings = r, r.Verdict, r.Fingerprint, r.Limits, r.Findings
	}
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
	fmt.Fprintf(stdout, "%s\nFingerprint: %s\n%s\n", verdict, fingerprint, limits)
	for _, finding := range findings {
		fmt.Fprintln(stdout, finding)
	}
	if verdict == "unavailable" {
		return 2
	}
	if verdict != "integrity-match" && verdict != "replay-match" {
		return 1
	}
	return 0
}
