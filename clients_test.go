package main

// Catalog snapshot/drift pattern adapted from Apache-2.0 MaveScape clients_test.go.
import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"testing"
)

var updateClients = flag.Bool("update-clients", false, "rewrite clients/tools.json; then run node clients/generate.mjs")

func TestTheClientsToolListIsCurrent(t *testing.T) {
	catalog := struct {
		API            int            `json:"api"`
		CatalogVersion int            `json:"catalogVersion"`
		PolicyVersion  int            `json:"policyVersion"`
		Tools          []remoteAction `json:"tools"`
	}{apiVersion, actionCatalogVersion, automationPolicyVersion, remoteActions}
	data, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if *updateClients {
		if err := os.WriteFile("clients/tools.json", data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	actual, err := os.ReadFile("clients/tools.json")
	if err != nil || !bytes.Equal(actual, data) {
		t.Fatal("Client catalog drift: go test -run '^TestTheClientsToolListIsCurrent$' -update-clients, then node clients/generate.mjs")
	}
}
