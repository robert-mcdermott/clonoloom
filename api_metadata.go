package main

import (
	"net/http"
	"strconv"

	"clonoloom/internal/airr"
	"clonoloom/internal/reads"
	"clonoloom/internal/rep"
)

// Explicit AIRR IDs always take precedence. Name matching only supports legacy
// tables without repertoire_id; it must not repair an unmatched explicit ID.
func repertoireMatch(sample rep.Sample, records map[string]*airr.Repertoire) (*airr.Repertoire, string) {
	if sample.RepertoireID != "" {
		return records[sample.RepertoireID], "repertoire_id"
	}
	return records[sample.Name], "sample name (legacy)"
}

type metadataRecord struct {
	Index          int    `json:"index"`
	RepertoireID   string `json:"repertoireId"`
	FileIndex      int    `json:"fileIndex"`
	MatchedSamples int    `json:"matchedSamples"`
	IDMatches      int    `json:"idMatches"`
	NameMatches    int    `json:"nameMatches"`
}

type metadataPage struct {
	Total                  int                  `json:"total"`
	MatchedRecords         int                  `json:"matchedRecords"`
	UnmatchedRecords       int                  `json:"unmatchedRecords"`
	SamplesWithMetadata    int                  `json:"samplesWithMetadata"`
	SamplesWithoutMetadata int                  `json:"samplesWithoutMetadata"`
	Offset                 int                  `json:"offset"`
	Limit                  int                  `json:"limit"`
	Files                  []reads.MetadataFile `json:"files"`
	Records                []metadataRecord     `json:"records"`
}

func (a *app) handleMetadata(w http.ResponseWriter, r *http.Request) {
	ds, ok := a.datasets.get(r.PathValue("id"))
	if !ok {
		writeErrorCode(w, http.StatusNotFound, "not-open", "The dataset is not open: open it again from its files.")
		return
	}
	// A detail response carries JSON as a string: browser JSON.parse never sees its
	// scientific numbers. Refuse an oversized preview rather than silently truncate.
	if raw, has := r.URL.Query()["record"]; has {
		if len(raw) != 1 {
			writeError(w, http.StatusBadRequest, "record must be a single index")
			return
		}
		index, err := strconv.Atoi(raw[0])
		if err != nil || index < 0 || index >= len(ds.Result.Repertoires) {
			writeError(w, http.StatusBadRequest, "record is outside this dataset's metadata")
			return
		}
		record := ds.Result.Repertoires[index]
		if len(record.Raw) > 4<<20 {
			writeError(w, http.StatusUnprocessableEntity, "This record exceeds the 4 MB inspection limit. Inspect the original metadata file; no evidence was truncated or changed.")
			return
		}
		writeJSON(w, map[string]any{"index": index, "rawJSON": string(record.Raw)})
		return
	}
	offset, limit := 0, 20
	for key, dst := range map[string]*int{"offset": &offset, "limit": &limit} {
		if raw, has := r.URL.Query()[key]; has {
			if len(raw) != 1 {
				writeError(w, http.StatusBadRequest, key+" must be a single integer")
				return
			}
			n, err := strconv.Atoi(raw[0])
			if err != nil {
				writeError(w, http.StatusBadRequest, key+" must be an integer")
				return
			}
			*dst = n
		}
	}
	reps := ds.Result.Repertoires
	if offset < 0 || offset > len(reps) || limit < 1 || limit > 50 {
		writeError(w, http.StatusBadRequest, "offset must be within the metadata and limit must be 1–50")
		return
	}
	p := metadataPage{Total: len(reps), Offset: offset, Limit: limit, Files: ds.Result.Metadata, Records: []metadataRecord{}}
	if p.Files == nil {
		p.Files = []reads.MetadataFile{}
	}
	lookup := make(map[string]*airr.Repertoire, len(reps))
	counts := make(map[string][2]int, len(reps))
	for i := range reps {
		lookup[reps[i].RepertoireID] = &reps[i]
	}
	for _, sample := range ds.Result.Table.Samples {
		record, basis := repertoireMatch(sample, lookup)
		if record == nil {
			p.SamplesWithoutMetadata++
			continue
		}
		p.SamplesWithMetadata++
		c := counts[record.RepertoireID]
		if basis == "repertoire_id" {
			c[0]++
		} else {
			c[1]++
		}
		counts[record.RepertoireID] = c
	}
	fileIndex, boundary := 0, 0
	for i, record := range reps {
		for fileIndex < len(p.Files) && i >= boundary+p.Files[fileIndex].Repertoires {
			boundary += p.Files[fileIndex].Repertoires
			fileIndex++
		}
		c := counts[record.RepertoireID]
		n := c[0] + c[1]
		if n == 0 {
			p.UnmatchedRecords++
		} else {
			p.MatchedRecords++
		}
		if i >= offset && i-offset < limit {
			p.Records = append(p.Records, metadataRecord{i, record.RepertoireID, fileIndex, n, c[0], c[1]})
		}
	}
	writeJSON(w, p)
}
