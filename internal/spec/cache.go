package spec

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// CachePath returns the path of the cached spec, `${XDG_CACHE_HOME:-~/.cache}/claude-complete/spec.json`.
func CachePath() (string, error) {
	dir := os.Getenv("XDG_CACHE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to find the cache directory: %w", err)
		}

		dir = filepath.Join(home, ".cache")
	}

	return filepath.Join(dir, "claude-complete", "spec.json"), nil
}

// Identify returns the resolved path, the size and the modification time of the binary called name on PATH,
// or "" when it is not found.
func Identify(name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return ""
	}

	if path, err = filepath.EvalSymlinks(path); err != nil {
		return ""
	}

	info, err := os.Stat(path)
	if err != nil {
		return ""
	}

	return fmt.Sprintf("%s %d %d", path, info.Size(), info.ModTime().UnixNano())
}

// Load returns the spec of the installed claude, from the cache at path when it is current, and from a
// fresh walk otherwise.
//
// The cache is current when it was read from the same binary, or else when claude reports the same
// version. Only the latter starts a process.
func Load(ctx context.Context, run Runner, path, binary string, refresh bool) (*Spec, error) {
	cached, err := read(path)
	usable := !refresh && err == nil && cached.Root.valid()

	if usable && binary != "" && cached.Binary == binary {
		return cached, nil
	}

	version, err := Version(ctx, run)
	if err != nil {
		return nil, err
	}

	if usable && cached.Version == version {
		// reinstalled or moved, the tree is the same
		cached.Binary = binary
		_ = write(path, cached)

		return cached, nil
	}

	root, err := Walk(ctx, run)
	if err != nil {
		return nil, err
	}

	fresh := &Spec{Version: version, Binary: binary, Root: root}

	_ = write(path, fresh) // failing only costs another walk

	return fresh, nil
}

func read(path string) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read the cached spec: %w", err)
	}

	var cached Spec
	if err = json.Unmarshal(data, &cached); err != nil {
		return nil, fmt.Errorf("failed to decode the cached spec: %w", err)
	}

	return &cached, nil
}

// write replaces the file at path atomically.
func write(path string, fresh *Spec) error {
	data, err := json.Marshal(fresh)
	if err != nil {
		return fmt.Errorf("failed to encode the spec: %w", err)
	}

	if err = os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create the cache directory: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".spec-*.json")
	if err != nil {
		return fmt.Errorf("failed to write the cached spec: %w", err)
	}

	defer os.Remove(tmp.Name()) // only left over on failure

	_, err = tmp.Write(data)
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}

	if err != nil {
		return fmt.Errorf("failed to write the cached spec: %w", err)
	}

	if err = os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("failed to replace the cached spec: %w", err)
	}

	return nil
}
