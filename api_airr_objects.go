package main

import (
	"net/http"
	"strconv"
)

func (a *app) handleAIRRObjects(w http.ResponseWriter, r *http.Request) {
	ds, ok := a.datasets.get(r.PathValue("id"))
	if !ok {
		writeError(w, 404, "Open the dataset again to inspect its captured AIRR objects.")
		return
	}
	parse := func(key string, def int) (int, bool) {
		v, ok := r.URL.Query()[key]
		if !ok {
			return def, true
		}
		if len(v) != 1 {
			return 0, false
		}
		n, err := strconv.Atoi(v[0])
		return n, err == nil
	}
	objects := ds.Result.Objects
	if _, ok := r.URL.Query()["record"]; ok {
		n, valid := parse("record", 0)
		if !valid || n < 0 || n >= len(objects) {
			writeError(w, 400, "record must be one valid object index")
			return
		}
		o := objects[n]
		if len(o.Raw) > 4<<20 {
			writeError(w, 422, "Object exceeds the 4 MiB preview limit; inspect its source file. Nothing was truncated.")
			return
		}
		f := ds.Result.Metadata[o.FileIndex]
		detail := map[string]any{"object": o, "rawJSON": string(o.Raw), "source": f, "format": f.Format}
		if len(f.SourceText) <= 4<<20 {
			detail["sourceText"] = f.SourceText
		} else {
			detail["sourceNotice"] = "Full source exceeds the 4 MiB preview limit; inspect the source file. No evidence was truncated."
		}
		writeJSON(w, detail)
		return
	}
	offset, ok1 := parse("offset", 0)
	limit, ok2 := parse("limit", 20)
	if !ok1 || !ok2 || offset < 0 || offset > len(objects) || limit < 1 || limit > 50 {
		writeError(w, 400, "offset must be within objects and limit must be 1–50")
		return
	}
	rows := []map[string]any{}
	for i := offset; i < len(objects) && i-offset < limit; i++ {
		o := objects[i]
		rows = append(rows, map[string]any{"index": i, "kind": o.Kind, "id": o.ID, "path": o.Path, "fileIndex": o.FileIndex, "links": len(o.Links)})
	}
	writeJSON(w, map[string]any{"total": len(objects), "offset": offset, "records": rows})
}
