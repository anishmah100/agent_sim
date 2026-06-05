package world

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Bundle is the v1 world-bundle manifest parsed from <dir>/bundle.toml.
// Each world ships as a self-contained directory at worlds/<name>/.
type Bundle struct {
	Dir string // absolute or relative path to the bundle directory

	Schema      string
	Name        string
	DisplayName string
	Description string

	WorldFile  string // relative to Dir
	ScenarioPkg string // engine-side scenario package name
	ArtPack    string // optional override; "" → use shared catalog
	NPCsFile   string // relative to Dir; "" → no default NPC supervisor
	Generator  string // relative to Dir; "" → none
}

// bundleTOML mirrors the on-disk layout. Fields are flat — TOML
// sections map cleanly onto nested structs.
type bundleTOML struct {
	Bundle struct {
		Schema      string `toml:"schema"`
		Name        string `toml:"name"`
		DisplayName string `toml:"display_name"`
		Description string `toml:"description"`
	} `toml:"bundle"`
	World struct {
		File string `toml:"file"`
	} `toml:"world"`
	Scenario struct {
		Pkg string `toml:"pkg"`
	} `toml:"scenario"`
	Art struct {
		Pack string `toml:"pack"`
	} `toml:"art"`
	NPCs struct {
		Config string `toml:"config"`
	} `toml:"npcs"`
	Design struct {
		Generator string `toml:"generator"`
	} `toml:"design"`
}

// ReadBundle parses bundle.toml from the given directory and returns the
// manifest. It does NOT load the world — call LoadBundle for that.
func ReadBundle(dir string) (*Bundle, error) {
	manifestPath := filepath.Join(dir, "bundle.toml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read bundle.toml at %s: %w", dir, err)
	}
	var t bundleTOML
	if _, err := toml.Decode(string(data), &t); err != nil {
		return nil, fmt.Errorf("parse bundle.toml at %s: %w", dir, err)
	}
	if t.Bundle.Schema != "agent_sim/bundle/v1" {
		return nil, fmt.Errorf("bundle.toml at %s: unsupported schema %q", dir, t.Bundle.Schema)
	}
	if t.Bundle.Name == "" {
		return nil, fmt.Errorf("bundle.toml at %s: bundle.name is required", dir)
	}
	if t.World.File == "" {
		return nil, fmt.Errorf("bundle.toml at %s: world.file is required", dir)
	}
	return &Bundle{
		Dir:         dir,
		Schema:      t.Bundle.Schema,
		Name:        t.Bundle.Name,
		DisplayName: t.Bundle.DisplayName,
		Description: t.Bundle.Description,
		WorldFile:   t.World.File,
		ScenarioPkg: t.Scenario.Pkg,
		ArtPack:     t.Art.Pack,
		NPCsFile:    t.NPCs.Config,
		Generator:   t.Design.Generator,
	}, nil
}

// LoadBundle is the one-call entry point: read bundle.toml + load world.json.
// Returns the World ready for Tick(), and the parsed Bundle metadata
// so the caller knows which scenario package to install and where to
// find the NPC supervisor config.
func LoadBundle(dir string) (*World, *Bundle, error) {
	b, err := ReadBundle(dir)
	if err != nil {
		return nil, nil, err
	}
	w, err := Load(filepath.Join(dir, b.WorldFile))
	if err != nil {
		return nil, b, err
	}
	return w, b, nil
}

// NPCsConfigPath returns the absolute (or bundle-relative) path to the
// NPC supervisor config, or "" if none is configured.
func (b *Bundle) NPCsConfigPath() string {
	if b == nil || b.NPCsFile == "" {
		return ""
	}
	return filepath.Join(b.Dir, b.NPCsFile)
}
