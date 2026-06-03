// Package npc supervises external NPC bot subprocesses.
//
// Per Q28 from session-2 decisions: NPCs in agent_sim connect via the
// same SDK as user-owned bots, running as subprocesses on the engine
// host. This package reads a JSON config, spawns the configured
// processes, restarts them when they exit (with backoff), and stops
// them cleanly on engine shutdown.
//
// The supervisor is intentionally dumb: it doesn't introspect
// observation/action traffic and it doesn't know what a "NPC" is at
// the world level. It just runs processes. The actual AI logic lives
// in the bot scripts themselves (examples/heuristic_bot.py and
// friends), which connect to /api/v1/agent/register like any other
// user-owned agent.
package npc

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// Spec describes one NPC entry from the supervisor config.
type Spec struct {
	// Name is a stable identifier used in logs + restart accounting.
	Name string `json:"name"`
	// Command is the executable path or name (resolved via $PATH).
	Command string `json:"command"`
	// Args are passed through to the child as argv[1:].
	Args []string `json:"args"`
	// Cwd is the working directory for the child. Empty = supervisor's cwd.
	Cwd string `json:"cwd,omitempty"`
	// Env is appended to the supervisor's environment.
	Env []string `json:"env,omitempty"`
	// Count is how many copies to spawn (each gets a "-i" suffix on Name
	// when Count > 1). Defaults to 1.
	Count int `json:"count,omitempty"`
	// AutoRestart, when true, restarts the child after exit. Backoff is
	// exponential from 500ms, capped at 30s. Default false.
	AutoRestart bool `json:"auto_restart,omitempty"`
}

// Config is the top-level config file format.
type Config struct {
	NPCs []Spec `json:"npcs"`
}

// LoadConfig reads + decodes a JSON config from disk.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &c, nil
}

// Supervisor runs and supervises the configured NPC processes.
type Supervisor struct {
	cfg     *Config
	logger  *log.Logger
	wg      sync.WaitGroup
	mu      sync.Mutex
	procs   []*managed
}

// New constructs a Supervisor. Pass a *log.Logger or nil for the
// default; the supervisor prefixes child stderr lines with the
// instance name.
func New(cfg *Config, logger *log.Logger) *Supervisor {
	if logger == nil {
		logger = log.Default()
	}
	return &Supervisor{cfg: cfg, logger: logger}
}

// Start launches every configured spec. Returns immediately; the
// supervisor goroutines run until ctx is cancelled. Calling Stop()
// after ctx is cancelled blocks until every child has exited.
func (s *Supervisor) Start(ctx context.Context) {
	for _, spec := range s.cfg.NPCs {
		count := spec.Count
		if count <= 0 {
			count = 1
		}
		for i := 0; i < count; i++ {
			name := spec.Name
			if count > 1 {
				name = fmt.Sprintf("%s-%d", spec.Name, i+1)
			}
			m := &managed{
				spec:    spec,
				name:    name,
				logger:  s.logger,
			}
			s.mu.Lock()
			s.procs = append(s.procs, m)
			s.mu.Unlock()
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				m.runLoop(ctx)
			}()
		}
	}
}

// Stop waits for every supervised process to exit. Cancel the context
// passed to Start first.
func (s *Supervisor) Stop() {
	s.wg.Wait()
}

// Stats returns a snapshot of per-process restart counts. Useful for
// /api/v1/world/npc/status if we wire one up later.
func (s *Supervisor) Stats() []Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Status, 0, len(s.procs))
	for _, m := range s.procs {
		out = append(out, m.Status())
	}
	return out
}

type Status struct {
	Name       string `json:"name"`
	Restarts   int    `json:"restarts"`
	LastExitAt string `json:"last_exit_at,omitempty"`
	Running    bool   `json:"running"`
}

// === per-process management ===

type managed struct {
	spec   Spec
	name   string
	logger *log.Logger

	mu         sync.Mutex
	restarts   int
	running    bool
	lastExit   time.Time
}

func (m *managed) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := Status{
		Name:     m.name,
		Restarts: m.restarts,
		Running:  m.running,
	}
	if !m.lastExit.IsZero() {
		s.LastExitAt = m.lastExit.UTC().Format(time.RFC3339)
	}
	return s
}

func (m *managed) runLoop(ctx context.Context) {
	backoff := 500 * time.Millisecond
	const maxBackoff = 30 * time.Second

	for {
		if ctx.Err() != nil {
			return
		}
		if err := m.runOnce(ctx); err != nil && ctx.Err() == nil {
			m.logger.Printf("npc[%s] exited: %v", m.name, err)
		}
		if !m.spec.AutoRestart {
			return
		}
		// Backoff before restarting.
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func (m *managed) runOnce(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, m.spec.Command, m.spec.Args...)
	if m.spec.Cwd != "" {
		cwd := m.spec.Cwd
		if !filepath.IsAbs(cwd) {
			if abs, err := filepath.Abs(cwd); err == nil {
				cwd = abs
			}
		}
		cmd.Dir = cwd
	}
	if len(m.spec.Env) > 0 {
		cmd.Env = append(os.Environ(), m.spec.Env...)
	}
	// Stream child stderr/stdout into the supervisor logger with a name
	// prefix so multi-NPC runs are debuggable.
	cmd.Stdout = &prefixWriter{prefix: "npc[" + m.name + "] ", out: m.logger.Writer()}
	cmd.Stderr = &prefixWriter{prefix: "npc[" + m.name + "] ", out: m.logger.Writer()}

	m.mu.Lock()
	m.running = true
	m.mu.Unlock()
	m.logger.Printf("npc[%s] starting: %s %v (cwd=%s)", m.name, m.spec.Command, m.spec.Args, cmd.Dir)

	err := cmd.Run()

	m.mu.Lock()
	m.running = false
	m.lastExit = time.Now()
	m.restarts++
	m.mu.Unlock()
	return err
}

// prefixWriter — io.Writer that prepends a prefix to every output line.
// Used to disambiguate logs from multiple NPC processes in one stream.
type prefixWriter struct {
	mu     sync.Mutex
	prefix string
	out    interface {
		Write(p []byte) (int, error)
	}
	pending []byte
}

func (w *prefixWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pending = append(w.pending, p...)
	for {
		idx := indexByte(w.pending, '\n')
		if idx < 0 {
			break
		}
		line := w.pending[:idx+1]
		w.pending = w.pending[idx+1:]
		buf := make([]byte, 0, len(w.prefix)+len(line))
		buf = append(buf, w.prefix...)
		buf = append(buf, line...)
		_, _ = w.out.Write(buf)
	}
	return len(p), nil
}

func indexByte(b []byte, c byte) int {
	for i, x := range b {
		if x == c {
			return i
		}
	}
	return -1
}
