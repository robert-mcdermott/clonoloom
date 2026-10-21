// The example datasets downloaded from their publishers as the program downloads them
// (internal/examples): every file checked against sources.json, a remote zip's members taken by
// range (the immuneACCESS archive is ZIP64), and each opening read as a dataset. It reaches the
// publishers, so it runs only when CLONOLOOM_FETCH_EXAMPLES is 1 (the maintainer, before a
// release), into a temporary folder; the data sets under terms of use are downloaded under the
// terms the maintainer accepted for validation (sources.json).
package validation

import (
	"context"
	"os"
	"testing"
	"time"

	"clonoloom/internal/examples"
	"clonoloom/internal/reads"
)

func TestExamplesDownloadFromThePublishers(t *testing.T) {
	if os.Getenv("CLONOLOOM_FETCH_EXAMPLES") != "1" {
		t.Skip("set CLONOLOOM_FETCH_EXAMPLES=1 to download the examples from their publishers")
	}
	raw, err := os.ReadFile("sources.json")
	if err != nil {
		t.Fatal(err)
	}
	sources, err := examples.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	lib := &examples.Library{Dir: t.TempDir(), Sources: sources, UserAgent: "ClonoLoom-validation"}
	for _, s := range sources {
		t.Run(s.Name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			start := time.Now()
			if _, err := lib.Download(ctx, s.Name, true, nil); err != nil {
				t.Fatal(err)
			}
			t.Logf("%s downloaded and checked in %s", s.Name, time.Since(start).Round(time.Millisecond))
			var state examples.State
			for _, st := range lib.List() {
				if st.Name == s.Name {
					state = st
				}
			}
			if state.State != "ready" || len(state.Paths) != len(s.Opens) {
				t.Fatalf("%+v", state)
			}
			for k, paths := range state.Paths {
				res, err := reads.Open(ctx, paths)
				if err != nil {
					t.Fatalf("%s: %v", s.Opens[k].Title, err)
				}
				if res.Table.Len() == 0 {
					t.Errorf("%s: no rows", s.Opens[k].Title)
				}
			}
		})
	}
}
