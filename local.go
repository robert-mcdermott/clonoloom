// Adapted from SituCraft 0.9.0 local.go.

package main

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// An item named on the command line (or by a later launch), or found in a folder named there: a
// file, or a folder ClonoLoom reads as one thing (a Cell Ranger output folder). Folder is the name
// of the folder it was found in, so the page can make a group of it.
type localFile struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Size   int64  `json:"size"`
	Folder string `json:"folder,omitempty"`
	Kind   string `json:"kind"`
	// Dir: a folder, read where it is by its reader, never served whole.
	Dir bool `json:"dir,omitempty"`
	// Path is where it is on this computer (the page records datasets by it; it never leaves
	// this computer).
	Path  string `json:"path"`
	path  string
	isDir bool
}

type localFiles struct {
	mu    sync.RWMutex
	files []localFile
	index map[string]int
}

const (
	// A folder of a cohort's exports rarely holds more than a few thousand files; the limit only
	// stops a mistaken folder (the home folder) from being opened.
	maxFolderFiles = 20000
	// A folder walk stops after this many entries, so the home folder does not walk for ever.
	maxFolderEntries = 200000
	// The file that makes a folder a Cell Ranger V(D)J output (vdj, or a multi run's vdj_t,
	// vdj_b and vdj_t_gd folders).
	contigManifest = "filtered_contig_annotations.csv"
)

func newLocalFiles() *localFiles {
	return &localFiles{index: map[string]int{}}
}

// fileKind classifies a file ClonoLoom can open by its name; "" means it cannot. Which reader a
// table needs (AIRR, Cell Ranger, immunoSEQ, MiXCR…) is decided from its header when it is read.
func fileKind(name string) string {
	lower := strings.ToLower(name)
	has := func(suffixes ...string) bool {
		for _, suffix := range suffixes {
			if strings.HasSuffix(lower, suffix) {
				return true
			}
		}
		return false
	}
	switch {
	case has(".clz"):
		return "project"
	case has(".tsv", ".tsv.gz", ".csv", ".csv.gz", ".txt", ".txt.gz", ".tab", ".tab.gz"):
		// Rearrangements, contigs, clonotypes, metrics, vendors' exports, specificity databases.
		return "table"
	case has(".json", ".json.gz", ".yaml", ".yml"):
		// AIRR Repertoire metadata and AIRR data files; Cell Ranger's contig annotations as JSON.
		return "metadata"
	case has(".h5ad", ".h5mu", ".h5"):
		// Gene expression with its cells (wave 6).
		return "expression"
	case has(".mtx", ".mtx.gz"):
		return "matrix"
	case has(".fasta", ".fa", ".fna", ".fasta.gz", ".fa.gz"):
		// Germline sets and sequences.
		return "fasta"
	case has(".zip"):
		return "archive"
	}
	return ""
}

// dirKind classifies a folder ClonoLoom reads as one thing; "" means its contents are looked at.
// A Cell Ranger run's folder, its outs/ and a multi run's per-sample vdj folders are one item each.
func dirKind(path string) string {
	if isFile(filepath.Join(path, contigManifest)) {
		return "cellranger"
	}
	if isFolder(filepath.Join(path, "per_sample_outs")) || isFile(filepath.Join(path, "outs", contigManifest)) || isFolder(filepath.Join(path, "outs", "per_sample_outs")) {
		return "cellranger"
	}
	return ""
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func isFolder(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func absPath(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "~" || strings.HasPrefix(name, "~/") || strings.HasPrefix(name, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			name = filepath.Join(home, name[1:])
		}
	}
	return filepath.Abs(name)
}

// add registers items and the openable items inside folders; it returns what it skipped.
func (l *localFiles) add(paths []string) []string {
	var problems []string
	var found []localFile
	for _, raw := range paths {
		if strings.TrimSpace(raw) == "" {
			// An empty path would silently mean the current folder: refuse it.
			problems = append(problems, "skipping an empty path")
			continue
		}
		path, err := absPath(raw)
		if err != nil {
			problems = append(problems, fmt.Sprintf("skipping %s: %v", raw, err))
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			problems = append(problems, fmt.Sprintf("skipping %s: %v", raw, err))
			continue
		}
		if info.IsDir() {
			if kind := dirKind(path); kind != "" {
				found = append(found, localFile{Name: info.Name(), Kind: kind, Dir: true, path: path, isDir: true})
				continue
			}
			items, problem := walkFolder(path)
			if problem != "" {
				problems = append(problems, problem)
			}
			if len(items) == 0 && problem == "" {
				problems = append(problems, fmt.Sprintf("skipping %s: no files ClonoLoom opens in the folder", raw))
			}
			found = append(found, items...)
			continue
		}
		kind := fileKind(info.Name())
		if kind == "" {
			problems = append(problems, fmt.Sprintf("skipping %s: not a file ClonoLoom opens", raw))
			continue
		}
		found = append(found, localFile{Name: info.Name(), Size: info.Size(), Kind: kind, path: path})
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, file := range found {
		if _, known := l.index[file.path]; known {
			continue
		}
		index := len(l.files)
		file.URL = "/api/local/" + strconv.Itoa(index)
		file.Path = file.path
		l.index[file.path] = index
		l.files = append(l.files, file)
	}
	return problems
}

// walkFolder finds the vendors' output folders and the openable files in a folder. An output
// folder is one item: the walk does not go inside it.
func walkFolder(dir string) ([]localFile, string) {
	folder := filepath.Base(dir)
	var files []localFile
	visited := 0
	problem := ""
	filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		visited++
		if visited > maxFolderEntries {
			problem = fmt.Sprintf("%s: stopped after %d entries", dir, maxFolderEntries)
			return filepath.SkipAll
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") && path != dir {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if len(files) >= maxFolderFiles {
			problem = fmt.Sprintf("%s: opened the first %d items", dir, maxFolderFiles)
			return filepath.SkipAll
		}
		if entry.IsDir() {
			if path == dir {
				return nil
			}
			if kind := dirKind(path); kind != "" {
				files = append(files, localFile{Name: name, Kind: kind, Folder: folder, Dir: true, path: path, isDir: true})
				return filepath.SkipDir
			}
			return nil
		}
		kind := fileKind(name)
		if kind == "" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		files = append(files, localFile{Name: name, Size: info.Size(), Kind: kind, Folder: folder, path: path})
		return nil
	})
	sort.SliceStable(files, func(i, j int) bool { return naturalLess(files[i].path, files[j].path) })
	return files, problem
}

// within answers the known items the paths name: each path's own item, or the items found inside a
// folder it names.
func (l *localFiles) within(paths []string) []localFile {
	var roots []string
	for _, raw := range paths {
		path, err := absPath(raw)
		if err != nil {
			continue
		}
		roots = append(roots, filepath.Clean(path))
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	var out []localFile
	for _, file := range l.files {
		for _, root := range roots {
			if file.path == root || strings.HasPrefix(file.path, root+string(filepath.Separator)) {
				out = append(out, file)
				break
			}
		}
	}
	return out
}

func (l *localFiles) list() []localFile {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]localFile, len(l.files))
	copy(out, l.files)
	return out
}

// get is the local item with this index.
func (l *localFiles) get(index string) (localFile, bool) {
	i, err := strconv.Atoi(index)
	l.mu.RLock()
	defer l.mu.RUnlock()
	if err != nil || i < 0 || i >= len(l.files) {
		return localFile{}, false
	}
	return l.files[i], true
}

func (l *localFiles) serve(w http.ResponseWriter, r *http.Request) {
	file, ok := l.get(r.PathValue("index"))
	if !ok {
		writeError(w, http.StatusNotFound, "No such local file.")
		return
	}
	if file.isDir {
		writeError(w, http.StatusBadRequest, file.Name+" is a folder: ClonoLoom reads it where it is.")
		return
	}
	handle, err := os.Open(file.path)
	if err != nil {
		writeError(w, http.StatusNotFound, "The file can no longer be read.")
		return
	}
	defer handle.Close()
	info, err := handle.Stat()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "The file can no longer be read.")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, file.Name, info.ModTime(), handle)
}

// naturalLess orders "region2" before "region10" and "slide 2" before "slide 10", as runs and
// samples are usually numbered.
func naturalLess(a, b string) bool {
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		ca, cb := a[i], b[j]
		if isDigit(ca) && isDigit(cb) {
			si := i
			for i < len(a) && isDigit(a[i]) {
				i++
			}
			sj := j
			for j < len(b) && isDigit(b[j]) {
				j++
			}
			na := strings.TrimLeft(a[si:i], "0")
			nb := strings.TrimLeft(b[sj:j], "0")
			if len(na) != len(nb) {
				return len(na) < len(nb)
			}
			if na != nb {
				return na < nb
			}
			continue
		}
		la, lb := lowerByte(ca), lowerByte(cb)
		if la != lb {
			return la < lb
		}
		i++
		j++
	}
	return len(a)-i < len(b)-j
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func lowerByte(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 32
	}
	return c
}
