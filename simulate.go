package main

// clonoloom simulate: a simulated receptor study with its truth (internal/simulate), written as AIRR
// rearrangement tables with their Repertoire metadata. It runs in the terminal and exits; nothing
// is served. Adapted from SituCraft 0.9.0 simulate.go.

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"clonoloom/internal/simulate"
)

// runSimulate runs `clonoloom simulate` with its arguments, answering the exit code.
func runSimulate(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("clonoloom simulate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var names []string
	for name := range simulate.Presets {
		names = append(names, name)
	}
	sort.Strings(names)
	preset := flags.String("preset", "standard", "the study to start from: "+strings.Join(names, ", "))
	seed := flags.Uint64("seed", 0, "the random seed (default: the preset's); the same parameters and seed give the same study, byte for byte")
	out := flags.String("out", "", "the folder to write (default: clonoloom-simulated-<preset>); it must not exist or be empty")
	paramsFile := flags.String("params", "", "a JSON file of the study's parameters, as --print-params writes them, replacing the preset's")
	printParams := flags.Bool("print-params", false, "print the study's parameters as JSON and exit")
	quiet := flags.Bool("quiet", false, "print only the summary")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: clonoloom simulate [flags]\n\nWrites a simulated TRB, paired B-cell or paired receptor/expression study whose truth is known: each sample an AIRR rearrangement\ntable, their Repertoire metadata, and the truth in its truth/ folder.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	study, ok := simulate.Presets[*preset]
	if !ok {
		fmt.Fprintf(stderr, "no preset %q: %s\n", *preset, strings.Join(names, ", "))
		return 2
	}
	if *paramsFile != "" {
		data, err := os.ReadFile(*paramsFile)
		if err == nil {
			study = simulate.Study{}
			err = json.Unmarshal(data, &study)
		}
		if err != nil {
			fmt.Fprintf(stderr, "--params: %v\n", err)
			return 2
		}
	}
	if *seed != 0 {
		study.Seed = *seed
	}
	if *printParams {
		data, _ := json.MarshalIndent(study, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return 0
	}
	if err := study.Check(); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	dir := *out
	if dir == "" {
		dir = "clonoloom-simulated-" + study.Name
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	last := ""
	summary, err := simulate.Run(ctx, study, dir, version, func(_ float64, message string) {
		if !*quiet && message != last && message != "Done" {
			fmt.Fprintln(stdout, message+"…")
			last = message
		}
	})
	if err != nil {
		fmt.Fprintf(stderr, "clonoloom simulate: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Wrote %s: %d subjects, %d samples, %d rearrangements (seed %d); the truth is in %s/truth.\nOpen it: clonoloom %s\n",
		dir, summary.Subjects, summary.Samples, summary.Rows, study.Seed, dir, dir)
	return 0
}
