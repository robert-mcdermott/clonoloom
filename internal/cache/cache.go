// Package cache keeps parsed rearrangement tables (clonoloom-spec/design.md, "The derived-data
// cache"), so that a dataset read once reopens without being read again: a cohort's hundred
// samples in a second or two rather than minutes. An entry is the table the readers made of a
// dataset's files, with its sources' records and the readers' problems, keyed by the readers'
// version and each file's SHA-256, size and name; a file that changed is a different key, so
// nothing stale is ever answered. Entries are written to a temporary name and renamed into place,
// carry a CRC-32 of their contents (an entry that fails it is removed and the files read again),
// and are removed least recently used first when the cache outgrows its limit. Sources are never
// written; the cache can be deleted at any time. Its size handling follows SituCraft 0.9.0
// internal/cache.
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"clonoloom/internal/reads"
)

// Cache is a folder of cached tables.
type Cache struct {
	root  string
	limit int64
	mu    sync.Mutex
	// read reads tables the cache does not hold (reads.ReadInputs; a test's counter).
	read reads.TableReader
}

// Open opens (making it if needed) the cache in dir, kept within limit bytes (0: no limit).
func Open(dir string, limit int64) (*Cache, error) {
	if err := os.MkdirAll(filepath.Join(dir, "tables"), 0o755); err != nil {
		return nil, err
	}
	return &Cache{root: dir, limit: limit, read: reads.ReadInputs}, nil
}

// Dir is the cache's folder.
func (c *Cache) Dir() string { return c.root }

// Limit is the size the cache is kept within (0: none).
func (c *Cache) Limit() int64 { return c.limit }

// key fingerprints the inputs as the readers see them: each file's bytes and its name (which
// names its sample when the file does not), with the readers' version.
func key(inputs []reads.Input) (string, []string, error) {
	h := sha256.New()
	fmt.Fprintf(h, "clonoloom-table-cache/%d readers %d\n", formatVersion, reads.ReaderVersion)
	sums := make([]string, len(inputs))
	for i, in := range inputs {
		f, err := os.Open(in.Path)
		if err != nil {
			return "", nil, err
		}
		fh := sha256.New()
		n, err := io.Copy(fh, f)
		f.Close()
		if err != nil {
			return "", nil, err
		}
		name := in.Name
		if name == "" {
			name = in.Path
		}
		sums[i] = hex.EncodeToString(fh.Sum(nil))
		fmt.Fprintf(h, "%s %d %s\n", sums[i], n, filepath.Base(name))
	}
	return hex.EncodeToString(h.Sum(nil)), sums, nil
}

func (c *Cache) entry(k string) string {
	return filepath.Join(c.root, "tables", k[:2], k+".clt")
}

// Tables reads a dataset's tables, from the cache when it holds them (a reads.TableReader).
func (c *Cache) Tables(ctx context.Context, inputs []reads.Input) (*reads.Result, error) {
	k, sums, err := key(inputs)
	if err != nil {
		return nil, err
	}
	path := c.entry(k)
	if res, err := load(path); err == nil {
		// The same bytes under the names and places of now.
		for i, in := range inputs {
			res.Sources[i].Path = in.Path
			name := in.Name
			if name == "" {
				name = in.Path
			}
			res.Sources[i].Name = filepath.Base(name)
			if info, err := os.Stat(in.Path); err == nil {
				res.Sources[i].Modified = info.ModTime().UTC().Format(time.RFC3339)
			}
		}
		now := time.Now()
		os.Chtimes(path, now, now)
		return res, nil
	} else if !os.IsNotExist(err) {
		os.Remove(path)
	}
	res, err := c.read(ctx, inputs)
	if err != nil {
		return nil, err
	}
	// Kept only when the files read are those fingerprinted (none changed in between).
	for i, s := range res.Sources {
		if s.SHA256 != sums[i] {
			return res, nil
		}
	}
	if err := c.put(path, k, res); err == nil {
		c.Evict()
	}
	return res, nil
}

func (c *Cache) put(path, k string, res *reads.Result) error {
	data, err := encode(k, res)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".clt-*")
	if err != nil {
		return err
	}
	_, err = temp.Write(data)
	if cerr := temp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(temp.Name(), path)
	}
	if err != nil {
		os.Remove(temp.Name())
	}
	return err
}

// Item is one cached table.
type Item struct {
	Name string    `json:"name"`
	Size int64     `json:"size"`
	Used time.Time `json:"used"`
	path string
}

// Usage is what the cache holds, most recently used first.
type Usage struct {
	Dir   string `json:"dir"`
	Limit int64  `json:"limit"`
	Size  int64  `json:"size"`
	Items []Item `json:"items"`
}

// Usage lists the cached tables.
func (c *Cache) Usage() (Usage, error) {
	u := Usage{Dir: c.root, Limit: c.limit}
	err := filepath.WalkDir(filepath.Join(c.root, "tables"), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".clt") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		u.Items = append(u.Items, Item{Name: strings.TrimSuffix(d.Name(), ".clt"), Size: info.Size(), Used: info.ModTime(), path: path})
		u.Size += info.Size()
		return nil
	})
	sort.Slice(u.Items, func(i, j int) bool { return u.Items[i].Used.After(u.Items[j].Used) })
	return u, err
}

// Evict removes the least recently used tables until the cache is within its limit.
func (c *Cache) Evict() ([]Item, error) {
	if c.limit <= 0 {
		return nil, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	u, err := c.Usage()
	if err != nil {
		return nil, err
	}
	var removed []Item
	for i := len(u.Items) - 1; i >= 0 && u.Size > c.limit; i-- {
		if err := os.Remove(u.Items[i].path); err != nil && !os.IsNotExist(err) {
			return removed, err
		}
		u.Size -= u.Items[i].Size
		removed = append(removed, u.Items[i])
	}
	return removed, nil
}

// Clear removes every cached table.
func (c *Cache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := os.RemoveAll(filepath.Join(c.root, "tables")); err != nil {
		return err
	}
	return os.MkdirAll(filepath.Join(c.root, "tables"), 0o755)
}

// ParseSize reads a size such as 10GB, 500MB or 0.
func ParseSize(s string) (int64, error) {
	t := strings.ToUpper(strings.TrimSpace(s))
	units := []struct {
		suffix string
		scale  float64
	}{{"TB", 1e12}, {"GB", 1e9}, {"MB", 1e6}, {"KB", 1e3}, {"T", 1e12}, {"G", 1e9}, {"M", 1e6}, {"K", 1e3}, {"B", 1}}
	scale := 1.0
	for _, u := range units {
		if strings.HasSuffix(t, u.suffix) {
			t, scale = strings.TrimSpace(strings.TrimSuffix(t, u.suffix)), u.scale
			break
		}
	}
	v, err := strconv.ParseFloat(t, 64)
	if err != nil || v < 0 || v*scale > 1e18 {
		return 0, fmt.Errorf("%q is not a size (for example 10GB or 500MB)", s)
	}
	return int64(v * scale), nil
}
