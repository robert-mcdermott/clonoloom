package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"clonoloom/internal/artifacts"
	"clonoloom/internal/certificate"
)

var archiveID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,79}$`)

// Source discovery is schema-bound: never infer an arbitrary path from prose or recipes.
func archiveSources(document string, raw []byte) ([]archiveSource, error) {
	var doc map[string]json.RawMessage
	if e := strictRunJSON(raw, &doc); e != nil {
		return nil, e
	}
	out := []archiveSource{}
	metadataBytes := 0
	for _, field := range []string{"datasets", "expression"} {
		var records []map[string]json.RawMessage
		if b := doc[field]; b != nil {
			if e := json.Unmarshal(b, &records); e != nil {
				return nil, e
			}
		}
		for i, record := range records {
			var credits []credit
			if b := record["credits"]; b != nil {
				if e := json.Unmarshal(b, &credits); e != nil {
					return nil, e
				}
			}
			for _, kind := range []string{"sources", "metadata"} {
				if field == "expression" && kind == "metadata" {
					continue
				}
				var sources []map[string]json.RawMessage
				if b := record[kind]; b != nil {
					if e := json.Unmarshal(b, &sources); e != nil {
						return nil, e
					}
				}
				for j, s := range sources {
					source := archiveSource{Document: document, Pointer: fmt.Sprintf("/%s/%d/%s/%d", field, i, kind, j), Credits: append([]credit{}, credits...), Status: "omitted", Reason: "Source inclusion was not requested."}
					if e := json.Unmarshal(s["path"], &source.Path); e != nil {
						return nil, errors.New("source needs a string path")
					}
					if b := s["sha256"]; b != nil {
						if e := json.Unmarshal(b, &source.SHA256); e != nil {
							return nil, e
						}
					}
					if b := s["size"]; b != nil {
						if e := json.Unmarshal(b, &source.Size); e != nil {
							return nil, e
						}
					}
					if b := s["name"]; b != nil {
						if e := json.Unmarshal(b, &source.Name); e != nil {
							return nil, e
						}
					}
					if source.Name == "" {
						source.Name = archiveBasename(source.Path)
					}
					metadataBytes += len(mustJSON(source))
					if metadataBytes > archiveRecordLimit {
						return nil, errors.New("source metadata exceeds 4 MiB")
					}
					out = append(out, source)
				}
			}
		}
	}
	if len(out) > 1024 {
		return nil, errors.New("at most 1024 source bindings per document")
	}
	return out, nil
}
func archiveRefs(raw []byte, key string) ([]string, error) {
	var doc map[string]json.RawMessage
	if e := strictRunJSON(raw, &doc); e != nil {
		return nil, e
	}
	var refs []struct {
		ID string `json:"id"`
	}
	if b := doc[key]; b != nil {
		if e := json.Unmarshal(b, &refs); e != nil {
			return nil, e
		}
	}
	ids := []string{}
	seen := map[string]bool{}
	for _, ref := range refs {
		if ref.ID == "" || seen[ref.ID] {
			return nil, errors.New("duplicate or empty " + key + " identity")
		}
		seen[ref.ID] = true
		ids = append(ids, ref.ID)
	}
	return ids, nil
}
func (a *app) collectArchive(ctx context.Context, raw []byte, opt archiveOptions) (archiveData, error) {
	d := archiveData{Files: map[string][]byte{}}
	if err := ctx.Err(); err != nil {
		return d, err
	}
	id, _, _, err := checkpointProject(raw)
	if err != nil {
		return d, err
	}
	d.Record = archiveRecord{Format: "clonoloom-project-archive", Version: 1, Project: id, Software: artifacts.Software{Name: "clonoloom", Version: version, Commit: buildCommit()}, Sources: []archiveSource{}, Dependencies: []archiveDependency{}, Limits: archiveLimits}
	if err = d.add("project.json", raw); err != nil {
		return d, err
	}
	documents := map[string][]byte{"project.json": raw}
	// Checkpoint closure retains immutable records and all their inventoried files.
	queue, _ := archiveRefs(raw, "checkpoints")
	seenCP := map[string]bool{}
	for len(queue) > 0 {
		cid := queue[0]
		queue = queue[1:]
		if seenCP[cid] {
			continue
		}
		seenCP[cid] = true
		if len(seenCP) > 16 {
			return d, errors.New("archive supports at most 16 checkpoints")
		}
		cp, e := a.loadCheckpoint(ctx, cid)
		if e != nil {
			return d, e
		}
		if cp.Record.Project != id {
			return d, errors.New("checkpoint belongs to another project")
		}
		record, e := archiveRead(ctx, filepath.Join(a.store.dir, "checkpoints", cid), "checkpoint.json", 1<<20)
		if e != nil {
			return d, e
		}
		if e = d.add("checkpoints/"+cid+"/checkpoint.json", record); e != nil {
			return d, e
		}
		for name, b := range cp.Files {
			if e = d.add("checkpoints/"+cid+"/"+name, b); e != nil {
				return d, e
			}
		}
		document := "checkpoints/" + cid + "/project.json"
		documents[document] = cp.Files["project.json"]
		refs, e := archiveRefs(cp.Files["project.json"], "checkpoints")
		if e != nil {
			return d, e
		}
		queue = append(queue, refs...)
	}
	// Attached results are mandatory. Available artifact parents are included recursively;
	// non-artifact parents remain in manifests; missing artifact parents have inventory statuses.
	ids, err := archiveRefs(raw, "artifacts")
	if err != nil {
		return d, err
	}
	mandatory := map[string]bool{}
	for _, id := range ids {
		mandatory[id] = true
	}
	seen := map[string]bool{}
	for len(ids) > 0 {
		aid := ids[0]
		ids = ids[1:]
		if seen[aid] {
			continue
		}
		seen[aid] = true
		if len(seen) > 128 {
			return d, errors.New("archive supports at most 128 result dependencies")
		}
		if _, e := a.artifacts.Path(aid, "result.json"); e != nil {
			return d, e
		}
		folder := filepath.Join(a.artifacts.Dir(), aid)
		manifest, e := archiveRead(ctx, folder, "manifest.json", certificate.MaxManifestBytes)
		if e != nil {
			if mandatory[aid] || !os.IsNotExist(e) {
				return d, fmt.Errorf("result %s unavailable: %w", aid, e)
			}
			d.Record.Dependencies = append(d.Record.Dependencies, archiveDependency{ID: aid, Kind: "artifact", Status: "missing"})
			continue
		}
		m, e := archiveManifest(manifest, aid)
		if e != nil {
			return d, e
		}
		if e = d.add("artifacts/"+aid+"/manifest.json", manifest); e != nil {
			return d, e
		}
		for _, f := range m.Files {
			b, e := archiveRead(ctx, folder, f.Path, f.Size)
			if e != nil {
				return d, e
			}
			if int64(len(b)) != f.Size || replayDigest(b) != f.SHA256 {
				return d, fmt.Errorf("changed artifact %s/%s", aid, f.Path)
			}
			if e = d.add("artifacts/"+aid+"/"+f.Path, b); e != nil {
				return d, e
			}
		}
		d.Record.Dependencies = append(d.Record.Dependencies, archiveDependency{ID: aid, Kind: m.Kind, Status: "included"})
		for _, p := range m.Lineage.Parents {
			if p.Kind == "dataset" || p.Kind == "expression" || p.Kind == "source" || p.ID == "" {
				continue
			}
			if _, e := a.artifacts.Path(p.ID, "result.json"); e == nil {
				ids = append(ids, p.ID)
			}
		}
	}
	// Prior immutable checkpoint documents still contain their original machine paths.
	// Re-export uses only their exact, verified imported source bindings as a fallback.
	portable := map[string]string{}
	var importedProject struct {
		PortableArchive string `json:"portableArchive"`
	}
	if json.Unmarshal(raw, &importedProject) == nil && importedProject.PortableArchive != "" {
		prior, e := a.loadImportedArchive(ctx, importedProject.PortableArchive)
		if e != nil {
			return d, e
		}
		for _, source := range prior.Record.Sources {
			if source.Status == "included" && string(documents[source.Document]) == string(prior.Files[source.Document]) {
				portable[source.Document+source.Pointer] = filepath.Join(a.store.dir, "imports", prior.Record.Fingerprint, filepath.FromSlash(source.Entry))
			}
		}
	}
	names := []string{}
	for name := range documents {
		names = append(names, name)
	}
	sort.Strings(names)
	sourceBytes := 0
	for _, document := range names {
		sources, e := archiveSources(document, documents[document])
		if e != nil {
			return d, e
		}
		for _, source := range sources {
			if a.examples != nil {
				source.Credits = append(source.Credits, a.credits([]string{source.SHA256})...)
			}
			restricted := false
			for _, c := range source.Credits {
				if c.LicenseClass != "open" {
					restricted = true
				}
			}
			switch {
			case restricted:
				source.Status = "restricted"
				source.Reason = "Provider terms or unknown provider license; source bytes are never bundled."
			case !opt.IncludeSources:
			case len(source.Credits) == 0 && !opt.OwnSources:
				source.Reason = "No open provider license recorded; explicit authority to include user-owned sources is required."
			case !runDigest(source.SHA256) || source.Size < 0 || source.Size > certificate.MaxFileBytes:
				source.Status = "unsupported"
				source.Reason = "Source needs a SHA-256 and bounded byte size."
			case source.Path == "":
				source.Status = "missing"
				source.Reason = "No local source binding."
			default:
				// Preserve original basenames for reader inference. Expression MEX companions share
				// one directory; all other inputs have independent fingerprinted directories.
				base := archiveBasename(source.Path)
				group := source.SHA256
				if strings.HasPrefix(source.Pointer, "/expression/") {
					parts := strings.Split(source.Pointer, "/")
					group = replayDigest([]byte(document + "/expression/" + parts[2]))
				}
				entry := "sources/" + group + "/" + base
				if !archivePath(entry) {
					source.Status = "unsupported"
					source.Reason = "Source filename is not portable across supported platforms."
					break
				}
				local := source.Path
				if bound := portable[source.Document+source.Pointer]; bound != "" {
					local = bound
				}
				file, e := archiveRead(ctx, filepath.Dir(local), filepath.Base(local), certificate.MaxFileBytes)
				if e != nil {
					if ctx.Err() != nil {
						return d, ctx.Err()
					}
					source.Status = "missing"
					source.Reason = "Source is missing, linked, non-regular or unreadable."
					break
				}
				if int64(len(file)) != source.Size || replayDigest(file) != source.SHA256 {
					source.Status = "changed"
					source.Reason = "Current bytes differ from the recorded source fingerprint."
					break
				}
				if e = d.add(entry, file); e != nil {
					return d, e
				}
				source.Entry = entry
				source.Status = "included"
				source.Reason = "Recorded fingerprint verified; source bytes included with permission."
			}
			sourceBytes += len(mustJSON(source))
			if sourceBytes > archiveRecordLimit {
				return d, errors.New("source metadata exceeds 4 MiB")
			}
			d.Record.Sources = append(d.Record.Sources, source)
		}
	}
	if len(d.Record.Sources) > 2048 {
		return d, errors.New("archive exceeds 2048 total source bindings")
	}
	// Captured evidence may itself contain source data. Retain the existing explicit
	// private-use gate even though restricted original source files are omitted.
	for name, b := range d.Files {
		if strings.HasSuffix(name, ".json") && archiveRestricted(b) {
			d.Record.Restricted = true
		}
	}
	for _, s := range d.Record.Sources {
		if s.Status == "restricted" {
			d.Record.Restricted = true
		}
	}
	if d.Record.Restricted && !opt.Accept {
		return d, errors.New("restricted evidence requires explicit private-use acknowledgment; source bytes remain excluded")
	}
	notice := "ClonoLoom portable project archive, version 1\n\n" + archiveLimits + "\n\n"
	if d.Record.Restricted {
		notice += "THESE DATA MAY NOT BE SHARED. Captured project/result evidence contains provider-restricted material; keep it within the permitted private study.\n\n"
	}
	for _, s := range d.Record.Sources {
		notice += s.Document + s.Pointer + ": " + s.Status + "; SHA-256 " + s.SHA256 + "; " + s.Reason + "\n"
		for _, c := range s.Credits {
			notice += "  " + c.Title + "; " + c.Source + "; " + c.License + "; " + c.LicenseURL + "; Cite: " + c.Citation + "\n"
		}
	}
	if e := d.add("NOTICE.txt", []byte(notice)); e != nil {
		return d, e
	}
	sort.Slice(d.Record.Dependencies, func(i, j int) bool { return d.Record.Dependencies[i].ID < d.Record.Dependencies[j].ID })
	archiveInventory(&d)
	if err = validateArchive(d); err != nil {
		return d, err
	}
	return d, nil
}

func validateArchive(d archiveData) error {
	if d.Record.Format != "clonoloom-project-archive" || d.Record.Version != 1 || len(d.Record.Dependencies) > 128 || len(mustJSON(d.Record)) > archiveRecordLimit {
		return errors.New("unsupported archive or inventory exceeds bounds")
	}
	raw := d.Files["project.json"]
	id, _, _, err := checkpointProject(raw)
	if err != nil {
		return err
	}
	if id != d.Record.Project {
		return errors.New("archive project identity mismatch")
	}
	if len(d.Files["NOTICE.txt"]) == 0 {
		return errors.New("missing archive notice")
	}
	used := map[string]bool{"project.json": true, "NOTICE.txt": true}
	documents := map[string][]byte{"project.json": raw}
	// Validate checkpoint records against their exact immutable inventories.
	for name, b := range d.Files {
		if !strings.HasPrefix(name, "checkpoints/") || !strings.HasSuffix(name, "/checkpoint.json") {
			continue
		}
		parts := strings.Split(name, "/")
		if len(parts) != 3 {
			return errors.New("invalid checkpoint path")
		}
		var record checkpointRecord
		if err := strictRunJSON(b, &record); err != nil {
			return err
		}
		if record.Format != "clonoloom-checkpoint" || record.Version != 1 || record.ID != parts[1] || checkpointID(record) != record.ID || record.Project != id {
			return errors.New("checkpoint identity or version mismatch")
		}
		used[name] = true
		prefix := "checkpoints/" + record.ID + "/"
		previous := ""
		for _, f := range record.Files {
			if !archivePath(f.Path) || f.Path <= previous || f.Path == "checkpoint.json" {
				return errors.New("invalid checkpoint inventory")
			}
			previous = f.Path
			data, ok := d.Files[prefix+f.Path]
			if !ok || int64(len(data)) != f.Size || replayDigest(data) != f.SHA256 {
				return errors.New("checkpoint evidence differs")
			}
			used[prefix+f.Path] = true
		}
		cp := d.Files[prefix+"project.json"]
		cid, _, _, e := checkpointProject(cp)
		if e != nil || cid != id {
			return errors.New("invalid checkpoint project")
		}
		// Match the checkpoint's complete snapshot schema, not just arbitrary hashed files.
		allowed := map[string]bool{"project.json": true}
		refs, e := archiveRefs(cp, "artifacts")
		if e != nil {
			return e
		}
		for _, aid := range refs {
			if !archiveID.MatchString(aid) {
				return errors.New("invalid checkpoint result identity")
			}
			manifest := "artifacts/" + aid + "/manifest.json"
			m, e := certificate.Manifest(d.Files[prefix+manifest])
			if e != nil || m.ID != aid {
				return errors.New("invalid checkpoint result manifest")
			}
			allowed[manifest] = true
			for _, f := range m.Files {
				name := "artifacts/" + aid + "/" + f.Path
				b, ok := d.Files[prefix+name]
				if !ok || int64(len(b)) != f.Size || replayDigest(b) != f.SHA256 {
					return errors.New("checkpoint result differs")
				}
				allowed[name] = true
			}
		}
		if len(allowed) != len(record.Files) || len(allowed) > 256 {
			return errors.New("checkpoint inventory differs from snapshot")
		}
		for _, f := range record.Files {
			if !allowed[f.Path] {
				return errors.New("unknown checkpoint member")
			}
		}
		documents[prefix+"project.json"] = cp
	}
	if len(documents) > 17 {
		return errors.New("too many checkpoints")
	}
	for _, doc := range documents {
		refs, e := archiveRefs(doc, "checkpoints")
		if e != nil {
			return e
		}
		for _, cid := range refs {
			if !used["checkpoints/"+cid+"/checkpoint.json"] {
				return errors.New("referenced checkpoint missing")
			}
		}
	}
	// Every global result is declared; every attached result is available.
	deps := map[string]bool{}
	for _, dep := range d.Record.Dependencies {
		if deps[dep.ID] || !archiveID.MatchString(dep.ID) || dep.ID == "staging" {
			return errors.New("invalid result dependencies")
		}
		deps[dep.ID] = true
		if dep.Status == "missing" {
			if len(d.Files["artifacts/"+dep.ID+"/manifest.json"]) != 0 {
				return errors.New("missing dependency has bytes")
			}
			continue
		}
		if dep.Status != "included" {
			return errors.New("invalid dependency status")
		}
		prefix := "artifacts/" + dep.ID + "/"
		m, e := archiveManifest(d.Files[prefix+"manifest.json"], dep.ID)
		if e != nil {
			return e
		}
		if m.Kind != dep.Kind {
			return errors.New("result kind mismatch")
		}
		used[prefix+"manifest.json"] = true
		for _, f := range m.Files {
			b, ok := d.Files[prefix+f.Path]
			if !ok || int64(len(b)) != f.Size || replayDigest(b) != f.SHA256 {
				return errors.New("artifact evidence differs")
			}
			used[prefix+f.Path] = true
		}
	}
	refs, e := archiveRefs(raw, "artifacts")
	if e != nil {
		return e
	}
	for _, aid := range refs {
		if !used["artifacts/"+aid+"/manifest.json"] {
			return errors.New("attached result missing")
		}
	}
	// Source records cannot introduce destinations or local reads absent from originals.
	expected := map[string]archiveSource{}
	for doc, b := range documents {
		ss, e := archiveSources(doc, b)
		if e != nil {
			return e
		}
		for _, s := range ss {
			expected[s.Document+s.Pointer] = s
		}
	}
	if len(expected) != len(d.Record.Sources) || len(expected) > 2048 {
		return errors.New("source inventory incomplete")
	}
	for _, s := range d.Record.Sources {
		key := s.Document + s.Pointer
		want, ok := expected[key]
		if !ok || want.Path != s.Path || want.Name != s.Name || want.SHA256 != s.SHA256 || want.Size != s.Size {
			return errors.New("source binding differs from original project")
		}
		delete(expected, key)
		switch s.Status {
		case "included":
			b, ok := d.Files[s.Entry]
			if !strings.HasPrefix(s.Entry, "sources/") || !archivePath(s.Entry) || !ok || int64(len(b)) != s.Size || replayDigest(b) != s.SHA256 {
				return errors.New("included source fingerprint differs")
			}
			for _, c := range s.Credits {
				if c.LicenseClass != "open" {
					return errors.New("restricted source bytes cannot be included")
				}
			}
			for _, c := range want.Credits {
				if c.LicenseClass != "open" {
					return errors.New("source policy differs from project")
				}
			}
			used[s.Entry] = true
		case "omitted", "restricted", "missing", "changed", "unsupported":
			if s.Entry != "" {
				return errors.New("omitted source has an entry")
			}
		default:
			return errors.New("invalid source status")
		}
	}
	for name := range d.Files {
		if !used[name] {
			return fmt.Errorf("unrecognized evidence member %s", name)
		}
	}
	restricted := false
	for name, b := range d.Files {
		if strings.HasSuffix(name, ".json") && archiveRestricted(b) {
			restricted = true
		}
	}
	for _, s := range d.Record.Sources {
		for _, c := range s.Credits {
			if c.LicenseClass != "open" {
				restricted = true
			}
		}
	}
	if restricted && !d.Record.Restricted {
		return errors.New("restricted evidence notice was removed")
	}
	return nil
}

// Patch known path fields as RawMessage; scalar scientific tokens are never decoded.
func bindArchiveProject(raw []byte, document, root string, r archiveRecord) ([]byte, error) {
	var doc map[string]json.RawMessage
	if e := strictRunJSON(raw, &doc); e != nil {
		return nil, e
	}
	for _, field := range []string{"datasets", "expression"} {
		if doc[field] == nil {
			continue
		}
		var records []map[string]json.RawMessage
		if e := json.Unmarshal(doc[field], &records); e != nil {
			return nil, e
		}
		for i, record := range records {
			for _, kind := range []string{"sources", "metadata"} {
				if field == "expression" && kind == "metadata" {
					continue
				}
				if record[kind] == nil {
					continue
				}
				var sources []map[string]json.RawMessage
				if e := json.Unmarshal(record[kind], &sources); e != nil {
					return nil, e
				}
				for j, source := range sources {
					pointer := "/" + field + "/" + strconv.Itoa(i) + "/" + kind + "/" + strconv.Itoa(j)
					found := false
					for _, s := range r.Sources {
						if s.Document != document || s.Pointer != pointer {
							continue
						}
						found = true
						local := ""
						if s.Status == "included" {
							local = filepath.Join(root, filepath.FromSlash(s.Entry))
						}
						source["path"] = mustJSON(local)
						source["archiveStatus"] = mustJSON(s.Status)
					}
					if !found {
						return nil, errors.New("source binding missing")
					}
				}
				record[kind] = mustJSON(sources)
			}
			if field == "expression" {
				var sources []struct {
					Path string `json:"path"`
					Role string `json:"role"`
				}
				json.Unmarshal(record["sources"], &sources)
				main, cells := "", ""
				complete := true
				for _, s := range sources {
					if s.Path == "" {
						complete = false
					}
					if s.Role == "cells" {
						cells = s.Path
					} else if main == "" {
						main = s.Path
					}
				}
				if !complete {
					main = ""
					cells = ""
				} else {
					for _, s := range sources {
						if s.Role == "matrix" {
							main = filepath.Dir(s.Path)
						}
					}
				}
				record["path"] = mustJSON(main)
				if record["cellsPath"] != nil {
					record["cellsPath"] = mustJSON(cells)
				}
			}
		}
		doc[field] = mustJSON(records)
	}
	doc["portableArchive"] = mustJSON(r.Fingerprint)
	return json.Marshal(doc)
}
