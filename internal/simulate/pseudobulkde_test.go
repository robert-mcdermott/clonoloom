package simulate

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestPseudobulkDEStudyIdentity(t *testing.T) {
	for _, name := range []string{"pseudobulk-de-study", "pseudobulk-de-null"} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "study")
			sum, e := Run(context.Background(), Presets[name], dir, "validation", nil)
			if e != nil {
				t.Fatal(e)
			}
			if sum.Subjects != 28 || sum.Samples != 28 || sum.Rows != 10466 {
				t.Fatal("rows", sum.Rows)
			}
			hash := sha256.New()
			for _, name := range sum.Files {
				raw, e := os.ReadFile(filepath.Join(dir, name))
				if e != nil {
					t.Fatal(e)
				}
				fmt.Fprintln(hash, name)
				hash.Write(raw)
			}
			digest := fmt.Sprintf("%x", hash.Sum(nil))
			want := map[string]string{"pseudobulk-de-study": "ee82857067124d03bf9652f59fb96bc0f82367e6d595e1a4e18bda17c69cd5e0", "pseudobulk-de-null": "de25fa97c96c2ea177f8e3e84075be07d0dd5a805255dfd8f907aa975f456324"}[name]
			if digest != want {
				t.Fatal("digest", digest)
			}
			var truth struct {
				Model string
				Files []bcellFile
			}
			raw, _ := os.ReadFile(filepath.Join(dir, "truth/truth.json"))
			if e = json.Unmarshal(raw, &truth); e != nil {
				t.Fatal(e)
			}
			if truth.Model != "clonoloom-artificial-pseudobulk-de-v1" {
				t.Fatal(truth.Model)
			}
			for _, f := range truth.Files {
				b, e := os.ReadFile(filepath.Join(dir, f.Path))
				if e != nil || fmt.Sprintf("%x", sha256.Sum256(b)) != f.SHA256 {
					t.Fatal("source hash", f.Path, e)
				}
			}
			if _, e = Run(context.Background(), Presets[name], dir, "validation", nil); e == nil {
				t.Fatal("overwrite")
			}
		})
	}
}
