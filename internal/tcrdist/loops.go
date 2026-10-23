package tcrdist

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Loops are the V alleles' CDR1, CDR2 and CDR2.5, aligned with gaps (.), by organism, as
// tcrdist3's database of germline genes has them (combo_xcr_2024-03-05.tsv, MIT; made from IMGT's
// germline genes): its cdrs column, the loops' amino acids at the alignment's columns (for human
// chains IMGT's 27–38, 56–65 and 81–86; tcrdist3's mouse α and β alignments place them a column or
// two later), taken as written rather than made again from IMGT's numbering, so that they are
// tcrdist3's. The database is downloaded on request (clonoloom-spec/roadmap.md, wave 4).
type Loops struct {
	// alleles[organism][allele]: the allele's loops.
	alleles map[string]map[string][3]string
	// order[organism]: the alleles in the file's order (a gene's first allele, for a call without
	// one).
	order map[string][]string
}

// ParseLoops reads tcrdist3's database: a tab-separated table with id (the allele), organism,
// region (V, D, J) and cdrs (the loops separated by ;, the CDR3's start last) among its columns.
func ParseLoops(r io.Reader) (*Loops, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<16), 1<<20)
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return nil, err
		}
		return nil, errors.New("the loops' database is empty")
	}
	col := map[string]int{}
	for i, h := range strings.Split(strings.TrimRight(sc.Text(), "\r"), "\t") {
		col[strings.TrimSpace(h)] = i
	}
	for _, name := range []string{"id", "organism", "region", "cdrs"} {
		if _, ok := col[name]; !ok {
			return nil, fmt.Errorf("the loops' database has no %s column: not tcrdist3's database of genes", name)
		}
	}
	l := &Loops{alleles: map[string]map[string][3]string{}, order: map[string][]string{}}
	for sc.Scan() {
		f := strings.Split(strings.TrimRight(sc.Text(), "\r"), "\t")
		get := func(name string) string {
			if i := col[name]; i < len(f) {
				return strings.TrimSpace(f[i])
			}
			return ""
		}
		if get("region") != "V" {
			continue
		}
		parts := strings.Split(get("cdrs"), ";")
		organism, id := get("organism"), get("id")
		if len(parts) < 3 || organism == "" || id == "" {
			continue
		}
		if l.alleles[organism] == nil {
			l.alleles[organism] = map[string][3]string{}
		}
		if _, dup := l.alleles[organism][id]; !dup {
			l.order[organism] = append(l.order[organism], id)
		}
		l.alleles[organism][id] = [3]string{parts[0], parts[1], parts[2]}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(l.alleles) == 0 {
		return nil, errors.New("the loops' database has no V genes with their loops")
	}
	return l, nil
}

// Organisms are the organisms the database has loops for, in order.
func (l *Loops) Organisms() []string {
	out := make([]string, 0, len(l.alleles))
	for o := range l.alleles {
		out = append(out, o)
	}
	sort.Strings(out)
	return out
}

// Alleles is how many V alleles an organism has loops for.
func (l *Loops) Alleles(organism string) int { return len(l.alleles[organism]) }

// Lookup answers a V call's loops in an organism, and the allele they are of. A call naming
// several genes (a tie) is taken as its first; a call without an allele, or with one the database
// lacks, as its gene's allele *01, else its gene's first allele in the database.
func (l *Loops) Lookup(organism, call string) (loops [3]string, allele string, ok bool) {
	genes := l.alleles[organism]
	if genes == nil {
		return loops, "", false
	}
	if i := strings.IndexAny(call, ",|"); i >= 0 {
		call = call[:i]
	}
	call = strings.TrimSpace(call)
	if call == "" {
		return loops, "", false
	}
	if loops, ok := genes[call]; ok {
		return loops, call, true
	}
	gene, _, _ := strings.Cut(call, "*")
	if loops, ok := genes[gene+"*01"]; ok {
		return loops, gene + "*01", true
	}
	for _, a := range l.order[organism] {
		if strings.HasPrefix(a, gene+"*") {
			return genes[a], a, true
		}
	}
	return loops, "", false
}

// Guess answers the organism whose database names the most of these V calls, with how many it
// names (human first among equals, then by name).
func (l *Loops) Guess(calls []string) (string, int) {
	best, found := "", -1
	for _, o := range append([]string{"human"}, l.Organisms()...) {
		if l.alleles[o] == nil {
			continue
		}
		n := 0
		for _, c := range calls {
			if c == "" {
				continue
			}
			if _, _, ok := l.Lookup(o, c); ok {
				n++
			}
		}
		if n > found {
			best, found = o, n
		}
	}
	return best, found
}
