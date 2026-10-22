package reads

import (
	"encoding/json"
	"fmt"
	"sort"
)

// ProblemCodes are every code a reader reports (the validator gives each a title and remedy).
var ProblemCodes = []string{
	"airr-missing-required", "bad-csv", "cell-count", "field-count", "gene-name-unmapped", "integer-as-decimal", "mixed-units",
	"molecules-missing", "negative-templates", "no-junction", "not-boolean", "not-integer", "not-number", "templates-missing",
	"unit-metadata",
}

// Problem is something a reader could not read as its format says, kept with where it was. The
// validator (internal/validate) turns problems into findings with remedies.
type Problem struct {
	Code    string `json:"code"`
	Source  int    `json:"source"`
	Line    uint32 `json:"line,omitempty"`
	Column  string `json:"column,omitempty"`
	Value   string `json:"value,omitempty"`
	Message string `json:"message"`
}

// Problems counts problems by code and keeps the first few of each as examples.
type Problems struct {
	counts   map[string]int
	examples map[string][]Problem
}

const examplesPerCode = 5

func newProblems() *Problems {
	return &Problems{counts: map[string]int{}, examples: map[string][]Problem{}}
}

// problemsJSON is Problems as JSON (the table cache keeps them with the table).
type problemsJSON struct {
	Counts   map[string]int       `json:"counts"`
	Examples map[string][]Problem `json:"examples"`
}

func (p *Problems) MarshalJSON() ([]byte, error) {
	return json.Marshal(problemsJSON{Counts: p.counts, Examples: p.examples})
}

func (p *Problems) UnmarshalJSON(data []byte) error {
	var v problemsJSON
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*p = Problems{counts: v.Counts, examples: v.Examples}
	if p.counts == nil {
		p.counts = map[string]int{}
	}
	if p.examples == nil {
		p.examples = map[string][]Problem{}
	}
	return nil
}

func (p *Problems) add(problem Problem) {
	p.counts[problem.Code]++
	if len(p.examples[problem.Code]) < examplesPerCode {
		if len(problem.Value) > 120 {
			problem.Value = problem.Value[:117] + "..."
		}
		p.examples[problem.Code] = append(p.examples[problem.Code], problem)
	}
}

func (p *Problems) addf(code string, source int, line uint32, column, value, format string, args ...any) {
	p.add(Problem{Code: code, Source: source, Line: line, Column: column, Value: value, Message: fmt.Sprintf(format, args...)})
}

// Summary is a problem code's count and examples.
type Summary struct {
	Code     string    `json:"code"`
	Count    int       `json:"count"`
	Examples []Problem `json:"examples"`
}

// Summaries in code order.
func (p *Problems) Summaries() []Summary {
	codes := make([]string, 0, len(p.counts))
	for code := range p.counts {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	out := make([]Summary, 0, len(codes))
	for _, code := range codes {
		out = append(out, Summary{Code: code, Count: p.counts[code], Examples: p.examples[code]})
	}
	return out
}

// Count of a code.
func (p *Problems) Count(code string) int { return p.counts[code] }
