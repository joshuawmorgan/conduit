// Package state persists run records. It provides an in-memory store and a
// JSON-file journal under a base directory (default .conduit/runs).
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/conduit-io/conduit/internal/model"
)

// Store persists and retrieves runs.
type Store interface {
	Save(run *model.Run) error
	Load(id string) (*model.Run, error)
	List() ([]*model.Run, error)
}

// Memory is a non-durable in-memory store.
type Memory struct {
	mu   sync.RWMutex
	runs map[string]*model.Run
}

// NewMemory returns an empty in-memory store.
func NewMemory() *Memory { return &Memory{runs: map[string]*model.Run{}} }

func (m *Memory) Save(run *model.Run) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *run
	m.runs[run.ID] = &cp
	return nil
}

func (m *Memory) Load(id string) (*model.Run, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.runs[id]
	if !ok {
		return nil, fmt.Errorf("run %q not found", id)
	}
	return r, nil
}

func (m *Memory) List() ([]*model.Run, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*model.Run, 0, len(m.runs))
	for _, r := range m.runs {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	return out, nil
}

// File is a durable JSON-file journal.
type File struct {
	dir string
	mu  sync.Mutex
}

// NewFile returns a file store rooted at dir, creating it if needed.
func NewFile(dir string) (*File, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}
	return &File{dir: dir}, nil
}

func (f *File) path(id string) string { return filepath.Join(f.dir, id+".json") }

func (f *File) Save(run *model.Run) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return err
	}
	tmp := f.path(run.ID) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, f.path(run.ID))
}

func (f *File) Load(id string) (*model.Run, error) {
	data, err := os.ReadFile(f.path(id))
	if err != nil {
		return nil, err
	}
	var run model.Run
	if err := json.Unmarshal(data, &run); err != nil {
		return nil, err
	}
	return &run, nil
}

func (f *File) List() ([]*model.Run, error) {
	entries, err := os.ReadDir(f.dir)
	if err != nil {
		return nil, err
	}
	var out []*model.Run
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		id := e.Name()[:len(e.Name())-len(".json")]
		if r, err := f.Load(id); err == nil {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	return out, nil
}
