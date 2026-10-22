// No fused multiply-adds in ClonoLoom's code (clonoloom-spec/conventions.md): Go may fuse a
// multiplication and the addition after it into one instruction on some processors (arm64) and
// not others (amd64), which rounds once instead of twice, so that a result differs between them in
// its last digits. The compiler's arm64 output of every package of the module is searched for
// fused instructions (FMADDD, FMSUBD, FNMADDD, FNMSUBD); each product added to something must be
// rounded on its own (float64(a*b)), which the Go specification says prevents fusion. The results
// themselves are pinned on both processors by TestResultsAreTheSameOnEveryProcessor and the
// simulator's TestTheSameStudyOnEveryProcessor.
package validation

import (
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestNoFusedMultiplyAdds(t *testing.T) {
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go command to compile with")
	}
	// The assembly of every package of the module (the standard library's is not ClonoLoom's); the
	// build cache replays it when nothing changed.
	cmd := exec.Command(gobin, "build", "-gcflags=clonoloom/...=-S", "./...")
	cmd.Dir = ".."
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=arm64", "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build for arm64: %v\n%s", err, out)
	}
	fused := regexp.MustCompile(`\(([^()]+\.go:\d+)\)\s+(FN?M(?:ADD|SUB)D)\b`)
	found := map[string]string{}
	functions := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, " STEXT ") {
			functions++
		}
		if m := fused.FindStringSubmatch(line); m != nil {
			at := m[1]
			if i := strings.Index(at, "/internal/"); i >= 0 {
				at = at[i+1:]
			}
			found[at] = m[2]
		}
	}
	if functions < 100 {
		t.Fatalf("only %d functions' assembly read: the compiler printed no listing?", functions)
	}
	var sites []string
	for at, op := range found {
		sites = append(sites, at+" "+op)
	}
	sort.Strings(sites)
	for _, s := range sites {
		t.Errorf("fused multiply-add at %s: round the product on its own, float64(a*b)", s)
	}
	t.Logf("%d functions' arm64 assembly searched, %d fused multiply-adds", functions, len(sites))
}
