package persist

import (
	"testing"

	"github.com/anishmah100/agent_sim/engine/internal/world"
)

func loadWorld(t *testing.T, path string) (*world.World, error) {
	t.Helper()
	return world.Load(path)
}
