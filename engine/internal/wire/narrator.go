package wire

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// NarratorHandler serves /api/v1/narrator/recent — the Story Feed UI
// (D15/D17) polls this for the most recent NarratorSummary records.
//
// The narrator runs as a SEPARATE process (tools/narrator) that tails
// the engine's event log and appends JSON lines to a file. This
// handler reads that file on demand and returns the last N records,
// optionally filtered by level. The engine doesn't generate narration
// itself; it just serves the file so the frontend has one origin
// (CORS) for everything.
//
// Query params:
//   - n     (int):    max records to return, newest-first. Default 30.
//   - level (str):    filter to one level (L1|L2|L3|L4). Default all.
//
// Response: {"events": [<record>, ...]} newest-first. Empty events
// array (NOT an error) when the file is missing — the narrator may
// simply not be running, and the Story Feed should render an empty
// state rather than an error toast.
func NarratorHandler(narratorPath string) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		rw.Header().Set("Access-Control-Allow-Origin", "*")
		rw.Header().Set("Cache-Control", "no-store")

		n := 30
		if v := r.URL.Query().Get("n"); v != "" {
			if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
				n = parsed
			}
		}
		level := strings.ToUpper(r.URL.Query().Get("level"))

		recs := readNarratorTail(narratorPath, n, level)
		_ = json.NewEncoder(rw).Encode(map[string]any{"events": recs})
	}
}

// readNarratorTail reads the jsonl file and returns up to n records
// (after level filtering) in newest-first order. Returns an empty
// (non-nil) slice when the file is absent or unreadable.
func readNarratorTail(path string, n int, level string) []json.RawMessage {
	out := []json.RawMessage{}
	if path == "" {
		return out
	}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()

	// Read all matching lines (the narrator file is small — at most a
	// few thousand short records per run). Keep only those matching the
	// level filter, then return the last n in newest-first order.
	var matched []json.RawMessage
	sc := bufio.NewScanner(f)
	// Allow long L4 closing-summary lines (default 64KB token limit is
	// too small for a multi-sentence Claude narrative).
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if level != "" {
			// Cheap level check before full parse.
			var probe struct {
				Level string `json:"level"`
			}
			if err := json.Unmarshal([]byte(line), &probe); err != nil {
				continue
			}
			if !strings.EqualFold(probe.Level, level) {
				continue
			}
		} else {
			// Still validate it's parseable JSON so we never emit
			// a half-written line.
			if !json.Valid([]byte(line)) {
				continue
			}
		}
		matched = append(matched, json.RawMessage(line))
	}

	// Take the last n, reverse to newest-first.
	start := 0
	if len(matched) > n {
		start = len(matched) - n
	}
	tail := matched[start:]
	for i := len(tail) - 1; i >= 0; i-- {
		out = append(out, tail[i])
	}
	return out
}
