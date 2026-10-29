// Package adc implements a bounded, user-initiated subset of ADC v1 metadata queries.
// Protocol: AIRR Standards 1.5.1 specs/adc-api.yaml (CC BY 4.0), API 1.2.0.
// See docs/ADC.md and clonoloom-spec/research.md §5.63; no provider implementation is used.
package adc

import (
	_ "embed"
	"strings"
)

const (
	Contract         = "ADC 1.2.0 /airr/v1 (AIRR Standards 1.5.1)"
	RegistryRevision = "4ae159f2799594a2c73669527f6fba97fc8995d7"
	RegistrySHA256   = "87e32b191050e6a37be24342afc65ea86a80be80b972b084a1d16cdf0b51f4e0"
	MaxPage          = 100
	MaxOffset        = 10000
	MaxResponse      = 8 << 20
)

//go:embed registry.tsv
var registry string

type Repository struct {
	ID      string `json:"id"`
	URL     string `json:"url"`
	Contact string `json:"contact"`
}

// Repositories returns a fresh copy of the pinned approved registry, never a remote lookup.
func Repositories() []Repository {
	out := []Repository{}
	for _, line := range strings.Split(strings.TrimSpace(registry), "\n")[1:] {
		parts := strings.Split(line, "\t")
		out = append(out, Repository{strings.TrimPrefix(parts[0], "https://"), parts[0], strings.TrimSpace(parts[1])})
	}
	return out
}

func repository(id string) (Repository, bool) {
	for _, r := range Repositories() {
		if r.ID == id {
			return r, true
		}
	}
	return Repository{}, false
}
