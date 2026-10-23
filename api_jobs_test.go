package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"clonoloom/internal/jobs"
)

// Resuming a previous method must not write new answers under the old method's artifact ID.
// Completed artifacts stay readable; an interrupted computation needs a new analysis.
func TestResumeRefusesAnEarlierAnalysisMethod(t *testing.T) {
	dir := t.TempDir()
	j := jobs.Job{ID: "j-old-neighborhood", Kind: "neighborhoods", Title: "Neighborhoods",
		State: jobs.Running, Created: time.Now(), Spec: json.RawMessage(`{"dataset":"d-test","method":1}`)}
	raw, err := json.Marshal(j)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, j.ID+".json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := jobs.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	a := &app{jobs: m}
	a.handleJobs()
	if _, err := m.Resume(j.ID); err == nil || !strings.Contains(err.Error(), "start a new analysis") {
		t.Fatalf("earlier method resumed: %v", err)
	}
	if kept, ok := m.Get(j.ID); !ok || kept.State != jobs.Orphaned {
		t.Fatalf("the interrupted job's record changed: %+v", kept)
	}
}
