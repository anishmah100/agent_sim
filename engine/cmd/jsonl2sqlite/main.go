// jsonl2sqlite — reads a historian events.jsonl + produces a SQLite
// database with indexed tables for fast queries:
//
//	events             — every event from the JSONL, columns:
//	                     id INTEGER PRIMARY KEY, tick, seq, kind,
//	                     category, payload (JSON text)
//	reasoning_traces   — denormalized view of category=agent_reasoning
//	                     rows: entity_id, action_id, verb, reasoning
//
// Indexes:
//	idx_events_tick      (tick)
//	idx_events_category  (category)
//	idx_events_kind      (kind)
//	idx_traces_entity    (reasoning_traces.entity_id)
//
// Usage:
//
//	go run ./cmd/jsonl2sqlite -in run.jsonl -out run.sqlite
//
// Idempotent — drops + recreates the tables so reruns are safe.
package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	_ "modernc.org/sqlite"
)

type Record struct {
	Tick     uint64          `json:"tick"`
	Seq      uint64          `json:"seq"`
	Kind     string          `json:"kind"`
	Category string          `json:"category"`
	Payload  json.RawMessage `json:"payload"`
}

type ReasoningPayload struct {
	EntityID  string `json:"entity_id"`
	ActionID  string `json:"action_id"`
	Verb      string `json:"verb"`
	Reasoning string `json:"reasoning"`
}

const schema = `
DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS reasoning_traces;

CREATE TABLE events (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    tick      INTEGER NOT NULL,
    seq       INTEGER NOT NULL,
    kind      TEXT NOT NULL,
    category  TEXT,
    payload   TEXT NOT NULL
);
CREATE INDEX idx_events_tick     ON events(tick);
CREATE INDEX idx_events_category ON events(category);
CREATE INDEX idx_events_kind     ON events(kind);

CREATE TABLE reasoning_traces (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    tick       INTEGER NOT NULL,
    seq        INTEGER NOT NULL,
    entity_id  TEXT NOT NULL,
    action_id  TEXT,
    verb       TEXT,
    reasoning  TEXT
);
CREATE INDEX idx_traces_entity ON reasoning_traces(entity_id);
CREATE INDEX idx_traces_tick   ON reasoning_traces(tick);
`

func main() {
	in := flag.String("in", "", "input JSONL file (historian output)")
	out := flag.String("out", "", "output SQLite path (created/recreated)")
	flag.Parse()
	if *in == "" || *out == "" {
		log.Fatalf("usage: -in run.jsonl -out run.sqlite")
	}

	// Remove pre-existing output so AUTOINCREMENT starts at 1.
	if err := os.Remove(*out); err != nil && !os.IsNotExist(err) {
		log.Fatalf("remove old sqlite: %v", err)
	}

	db, err := sql.Open("sqlite", *out)
	if err != nil {
		log.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(schema); err != nil {
		log.Fatalf("schema: %v", err)
	}

	src, err := os.Open(*in)
	if err != nil {
		log.Fatalf("open jsonl: %v", err)
	}
	defer src.Close()

	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("begin: %v", err)
	}
	insertEv, err := tx.Prepare("INSERT INTO events (tick, seq, kind, category, payload) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		log.Fatalf("prep event: %v", err)
	}
	insertTrace, err := tx.Prepare("INSERT INTO reasoning_traces (tick, seq, entity_id, action_id, verb, reasoning) VALUES (?, ?, ?, ?, ?, ?)")
	if err != nil {
		log.Fatalf("prep trace: %v", err)
	}

	scanner := bufio.NewScanner(src)
	scanner.Buffer(make([]byte, 0, 1<<20), 16<<20)
	rows := 0
	traces := 0
	for scanner.Scan() {
		var r Record
		if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
			continue
		}
		if _, err := insertEv.Exec(r.Tick, r.Seq, r.Kind, r.Category, string(r.Payload)); err != nil {
			log.Fatalf("insert event: %v", err)
		}
		rows++
		if r.Category == "agent_reasoning" {
			var rp ReasoningPayload
			if err := json.Unmarshal(r.Payload, &rp); err == nil {
				_, err := insertTrace.Exec(r.Tick, r.Seq, rp.EntityID, rp.ActionID, rp.Verb, rp.Reasoning)
				if err != nil {
					log.Fatalf("insert trace: %v", err)
				}
				traces++
			}
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("scan: %v", err)
	}
	if err := tx.Commit(); err != nil {
		log.Fatalf("commit: %v", err)
	}
	fmt.Printf("wrote %s — %d events, %d reasoning traces\n", *out, rows, traces)
}
