package bcell

// B-cell phylogenies: Fitch 1971, doi:10.1093/sysbio/20.4.406;
// Sankoff 1975, doi:10.1137/0128004. Metadata are observations, not inferred
// migration or class switching (Hoehn et al. 2022, doi:10.1371/journal.pcbi.1009885).
import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"clonoloom/internal/rep"
	"clonoloom/internal/study"
)

const LineageVersion = 1
const LineageDescription = "Equal-cost nucleotide parsimony within each saved family/locus, rooted at its V/J reference on a shared, complete-case reference-coordinate panel. D/N/P, gaps, ambiguity and uncovered sites are omitted; identical panel states collapse without abundance weighting. At most 8 taxa including the reference are searched exhaustively; 9–64 use three deterministic addition orders and strict-improvement NNI, capped at 50000 candidate scores and 200 million pattern-node work units. Heuristic scores are not proven global minima. One reproducible binary topology and one consistent minimum-change ancestral assignment are shown, with marginal IUPAC state sets conditional on that topology. Exact split counts describe tied optimal binary trees, not bootstrap support or probabilities. Zero-length edges and binary resolutions do not establish distinct ancestors. Observed isotypes, timepoints and tissues do not constrain topology or infer switching, migration or event dates."

type LineageColumn struct {
	Segment  string `json:"segment"`
	Position int    `json:"position"`
}
type LineageMember struct {
	FamilyRow
	Family      string `json:"family"`
	Observation string `json:"observation"`
	Sample      string `json:"sample"`
	Isotype     string `json:"isotype"`
	Timepoint   string `json:"timepoint"`
	Tissue      string `json:"tissue"`
	Tree        string `json:"tree"`
	Node        string `json:"node,omitempty"`
	Excluded    string `json:"excluded,omitempty"`
}
type LineageAnnotation struct {
	Sample    string `json:"sample"`
	Isotype   string `json:"isotype"`
	Timepoint string `json:"timepoint"`
	Tissue    string `json:"tissue"`
	Rows      int    `json:"rows"`
}
type LineageNode struct {
	ID          string              `json:"id"`
	Parent      string              `json:"parent,omitempty"`
	Kind        string              `json:"kind"`
	Sequence    string              `json:"sequence"`
	Possible    string              `json:"possible"`
	Changes     int                 `json:"changes"`
	Depth       int                 `json:"depth"`
	Distance    int                 `json:"distance"`
	Rows        int                 `json:"rows"`
	Annotations []LineageAnnotation `json:"annotations"`
	SplitTrees  *int                `json:"splitTrees,omitempty"`
}
type LineageTree struct {
	ID            string          `json:"id"`
	Family        string          `json:"family"`
	Locus         string          `json:"locus"`
	V             string          `json:"v"`
	J             string          `json:"j"`
	Rows          int             `json:"rows"`
	Included      int             `json:"included"`
	Sites         int             `json:"sites"`
	Omitted       int             `json:"omitted"`
	Taxa          int             `json:"taxa"`
	Score         int             `json:"score"`
	Search        string          `json:"search"`
	Examined      int             `json:"examined"`
	OptimalTrees  *int            `json:"optimalTrees,omitempty"`
	BudgetLimited bool            `json:"budgetLimited"`
	Excluded      string          `json:"excluded,omitempty"`
	Columns       []LineageColumn `json:"columns,omitempty"`
	Nodes         []LineageNode   `json:"nodes,omitempty"`
	Newick        string          `json:"newick,omitempty"`
}
type LineageResult struct {
	Trees       []LineageTree   `json:"trees"`
	Members     []LineageMember `json:"members"`
	Built       int             `json:"built"`
	Excluded    map[string]int  `json:"excluded"`
	Description string          `json:"description"`
}
type referenceBase struct{ query, reference byte }

func lineagePanelRow(r GermlineRow) (map[LineageColumn]referenceBase, string) {
	out := map[LineageColumn]referenceBase{}
	n := len(r.Query)
	if n == 0 || n > 2*maxGermlineLength || len(r.Germline) != n || len(r.Regions) != n || len(r.ReferencePositions) != n || len(r.QueryPositions) != n {
		return nil, "invalid saved alignment or coordinate maps"
	}
	for i := range r.Query {
		segment := r.Regions[i]
		if segment == 'N' {
			continue
		}
		if segment != 'V' && segment != 'J' {
			return nil, "invalid saved segment labels"
		}
		p := r.ReferencePositions[i]
		if p < 0 || p > maxGermlineLength {
			return nil, "invalid saved reference coordinates"
		}
		if p == 0 {
			if r.Germline[i] != '-' {
				return nil, "invalid saved insertion mapping"
			}
			continue
		}
		key := LineageColumn{string(segment), p}
		if _, ok := out[key]; ok {
			return nil, "duplicate reference coordinate"
		}
		out[key] = referenceBase{r.Query[i], r.Germline[i]}
	}
	return out, ""
}
func Lineages(ctx context.Context, t *rep.Table, sources []FamilySource, parent GermlineResult, annotations []study.Sample) (LineageResult, error) {
	out := LineageResult{Trees: []LineageTree{}, Members: []LineageMember{}, Excluded: map[string]int{}, Description: LineageDescription}
	indices := map[string]int{}
	for i := 0; i < t.Len(); i++ {
		if i%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return out, err
			}
		}
		if int(t.Source[i]) >= len(sources) {
			return out, errors.New("source index outside dataset")
		}
		id := familyID("bcr-row-", []any{sources[t.Source[i]], t.Line[i], t.IDs.Get(t.SequenceID[i])})
		if _, ok := indices[id]; ok {
			return out, errors.New("duplicate source row identity")
		}
		indices[id] = i
	}
	ann := map[string]study.Sample{}
	for _, a := range annotations {
		if _, ok := ann[a.Name]; ok {
			return out, errors.New("duplicate sample annotation")
		}
		ann[a.Name] = a
	}
	groups := map[string][]int{}
	seen := map[string]bool{}
	for k, g := range parent.Rows {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		i, ok := indices[g.ID]
		if !ok || seen[g.ID] || g.Source != sources[t.Source[i]] || g.Line != t.Line[i] || g.SequenceID != t.IDs.Get(t.SequenceID[i]) || g.Locus != t.LocusName(i) || g.Sample != t.Samples[t.Sample[i]].Name {
			return out, errors.New("saved germline rows do not match the open dataset")
		}
		seen[g.ID] = true
		a := ann[g.Sample]
		r := LineageMember{FamilyRow: g.FamilyRow, Family: g.Family, Observation: g.Observation, Sample: g.Sample, Isotype: t.Dict.Get(t.C[i]), Timepoint: a.Time, Tissue: a.Tissue, Excluded: g.Excluded}
		out.Members = append(out.Members, r)
		key := g.Family + "\x00" + g.Locus
		groups[key] = append(groups[key], k)
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		members := groups[key]
		first := parent.Rows[members[0]]
		tree := LineageTree{ID: familyID("tree-", []any{first.Family, first.Locus}), Family: first.Family, Locus: first.Locus, V: first.V, J: first.J, Rows: len(members)}
		eligible := []int{}
		union := map[LineageColumn]bool{}
		common := map[LineageColumn]referenceBase{}
		for _, k := range members {
			if err := ctx.Err(); err != nil {
				return out, err
			}
			out.Members[k].Tree = tree.ID
			if out.Members[k].Excluded != "" {
				continue
			}
			// Only one row's coordinate map is live here. Do not duplicate the
			// parent's entire alignment collection as maps for a large cohort.
			p, why := lineagePanelRow(parent.Rows[k])
			if why != "" {
				out.Members[k].Excluded = why
				continue
			}
			if len(eligible) == 0 {
				tree.V, tree.J = parent.Rows[k].V, parent.Rows[k].J
			}
			if parent.Rows[k].V != tree.V || parent.Rows[k].J != tree.J {
				tree.Excluded = "conflicting saved family alleles"
			}
			for col := range p {
				union[col] = true
			}
			if len(eligible) == 0 {
				for col, b := range p {
					if canonical(b.query) && canonical(b.reference) {
						common[col] = b
					}
				}
			} else {
				for col, b := range common {
					next, ok := p[col]
					if ok && canonical(next.reference) && b.reference != next.reference {
						tree.Excluded = "conflicting saved reference bases"
					}
					if !ok || !canonical(next.query) || !canonical(next.reference) || next.reference != b.reference {
						delete(common, col)
					}
				}
			}
			eligible = append(eligible, k)
		}
		tree.Sites = len(common)
		tree.Omitted = len(union) - tree.Sites
		if len(eligible) == 0 {
			tree.Excluded = "no reconstructed source rows"
		} else if tree.Sites == 0 {
			tree.Excluded = "no shared evaluable V/J reference positions"
		}
		if tree.Excluded == "" {
			for col := range common {
				tree.Columns = append(tree.Columns, col)
			}
			sort.Slice(tree.Columns, func(i, j int) bool {
				a, b := tree.Columns[i], tree.Columns[j]
				if a.Segment != b.Segment {
					return a.Segment == "V"
				}
				return a.Position < b.Position
			})
			var reference strings.Builder
			for _, col := range tree.Columns {
				reference.WriteByte(common[col].reference)
			}
			bySequence := map[string][]int{}
			for _, k := range eligible {
				if err := ctx.Err(); err != nil {
					return out, err
				}
				p, _ := lineagePanelRow(parent.Rows[k]) // Validated in the intersection pass.
				var q strings.Builder
				for _, col := range tree.Columns {
					q.WriteByte(p[col].query)
				}
				bySequence[q.String()] = append(bySequence[q.String()], k)
			}
			sequences := []string{reference.String()}
			for q := range bySequence {
				if q != sequences[0] {
					sequences = append(sequences, q)
				}
			}
			sort.Strings(sequences[1:])
			tree.Taxa = len(sequences)
			if tree.Taxa > LineageMaxTaxa {
				tree.Excluded = "more than 64 distinct panel states including the reference"
				tree.Columns = nil
			} else {
				result, err := searchTree(ctx, sequences, false)
				if errors.Is(err, errLineageWork) {
					tree.Excluded = err.Error()
					tree.Columns = nil
					for _, k := range members {
						if out.Members[k].Excluded == "" {
							out.Members[k].Excluded = tree.Excluded
						}
						out.Excluded[out.Members[k].Excluded]++
					}
					out.Trees = append(out.Trees, tree)
					continue
				}
				if err != nil {
					return out, err
				}
				tree.Score, tree.Examined, tree.BudgetLimited = result.score, result.examined, result.limited
				tree.Search = "heuristic addition/NNI"
				if result.exact {
					tree.Search = "exhaustive"
					n := result.optima
					tree.OptimalTrees = &n
				}
				chosen, possible, lengths := ancestralStates(result.graph, sequences)
				parents, order := result.graph.rooted()
				clades := result.graph.clades(len(sequences))
				ids := make([]string, len(result.graph))
				for i := range ids {
					ids[i] = familyID("node-", []any{tree.ID, clades[i], i == 0})
				}
				depth, distance := make([]int, len(ids)), make([]int, len(ids))
				for _, u := range order {
					node := LineageNode{ID: ids[u], Kind: "inferred", Sequence: chosen[u], Possible: possible[u], Changes: lengths[u], Annotations: []LineageAnnotation{}}
					if u == 0 {
						node.Kind = "reference"
					} else if u < len(sequences) {
						node.Kind = "observed"
					}
					if parents[u] >= 0 {
						node.Parent = ids[parents[u]]
						depth[u] = depth[parents[u]] + 1
						distance[u] = distance[parents[u]] + lengths[u]
					}
					node.Depth, node.Distance = depth[u], distance[u]
					if result.exact && u != 0 {
						n := result.support[clades[u]]
						node.SplitTrees = &n
					}
					if u < len(sequences) {
						counts := map[string]*LineageAnnotation{}
						for _, k := range bySequence[sequences[u]] {
							out.Members[k].Node = node.ID
							node.Rows++
							r := out.Members[k]
							a := LineageAnnotation{Sample: r.Sample, Isotype: r.Isotype, Timepoint: r.Timepoint, Tissue: r.Tissue}
							key := familyID("", a)
							if counts[key] == nil {
								counts[key] = &a
							}
							counts[key].Rows++
						}
						var akeys []string
						for k := range counts {
							akeys = append(akeys, k)
						}
						sort.Strings(akeys)
						for _, k := range akeys {
							node.Annotations = append(node.Annotations, *counts[k])
						}
					}
					tree.Nodes = append(tree.Nodes, node)
				}
				// Newick has a reference root with one descendant subtree. Branch lengths
				// are changes in the chosen assignment; zero edges are intentionally retained.
				var newick func(int) string
				newick = func(u int) string {
					var children []string
					for _, v := range result.graph[u] {
						if v != parents[u] {
							children = append(children, newick(v))
						}
					}
					sort.Strings(children)
					s := ids[u]
					if len(children) > 0 {
						s = "(" + strings.Join(children, ",") + ")" + s
					}
					if parents[u] >= 0 {
						s += ":" + strconv.Itoa(lengths[u])
					}
					return s
				}
				tree.Newick = newick(0) + ";"
				tree.Included = len(eligible)
				out.Built++
			}
		}
		for _, k := range members {
			if out.Members[k].Excluded == "" && tree.Excluded != "" {
				out.Members[k].Excluded = tree.Excluded
			}
			if why := out.Members[k].Excluded; why != "" {
				out.Excluded[why]++
			}
		}
		out.Trees = append(out.Trees, tree)
	}
	sort.Slice(out.Members, func(i, j int) bool { return out.Members[i].ID < out.Members[j].ID })
	return out, nil
}
func (t LineageTree) Summary() LineageTree { t.Nodes = nil; t.Columns = nil; t.Newick = ""; return t }
func (t LineageTree) Label() string        { return fmt.Sprintf("%s %s", t.Family, t.Locus) }
