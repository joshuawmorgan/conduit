// Package plugin manages the lifecycle of external Conduit plugins: discovery,
// launch, invocation and shutdown. It implements runtime.ActionInvoker.
package plugin

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	hclog "github.com/hashicorp/go-hclog"
	hplugin "github.com/hashicorp/go-plugin"

	pkgplugin "github.com/conduit-io/conduit/pkg/plugin"
)

// pluginPrefix is the required filename prefix for discovered plugin binaries.
const pluginPrefix = "conduit-plugin-"

// Manager discovers and dispenses plugins.
type Manager struct {
	mu         sync.Mutex
	dirs       []string
	discovered map[string]string // name -> executable path
	clients    map[string]*hplugin.Client
	providers  map[string]pkgplugin.ActionProvider
	logger     hclog.Logger
}

// New creates a Manager and scans dirs for plugin executables.
func New(dirs []string) *Manager {
	m := &Manager{
		dirs:       dirs,
		discovered: map[string]string{},
		clients:    map[string]*hplugin.Client{},
		providers:  map[string]pkgplugin.ActionProvider{},
		logger: hclog.New(&hclog.LoggerOptions{
			Name:   "conduit.plugin",
			Level:  hclog.Warn,
			Output: io.Discard,
		}),
	}
	m.scan()
	return m
}

func (m *Manager) scan() {
	for _, dir := range m.dirs {
		entries, err := filepath.Glob(filepath.Join(dir, pluginPrefix+"*"))
		if err != nil {
			continue
		}
		for _, path := range entries {
			base := filepath.Base(path)
			if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(base), ".exe") {
				continue
			}
			name := strings.TrimPrefix(base, pluginPrefix)
			name = strings.TrimSuffix(name, ".exe")
			if name == "" {
				continue
			}
			if _, exists := m.discovered[name]; !exists {
				m.discovered[name] = path
			}
		}
	}
}

// Discovered returns the names of plugins found on disk.
func (m *Manager) Discovered() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	names := make([]string, 0, len(m.discovered))
	for n := range m.discovered {
		names = append(names, n)
	}
	return names
}

// ensure launches (once) the plugin providing the given name.
func (m *Manager) ensure(name string) (pkgplugin.ActionProvider, error) {
	if p, ok := m.providers[name]; ok {
		return p, nil
	}
	path, ok := m.discovered[name]
	if !ok {
		return nil, fmt.Errorf("plugin %q not found in %v", name, m.dirs)
	}
	client := hplugin.NewClient(&hplugin.ClientConfig{
		HandshakeConfig:  pkgplugin.Handshake,
		Plugins:          pkgplugin.PluginSet(),
		Cmd:              exec.Command(path),
		Logger:           m.logger,
		AllowedProtocols: []hplugin.Protocol{hplugin.ProtocolNetRPC},
	})
	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("start plugin %q: %w", name, err)
	}
	raw, err := rpcClient.Dispense("action")
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("dispense plugin %q: %w", name, err)
	}
	provider, ok := raw.(pkgplugin.ActionProvider)
	if !ok {
		client.Kill()
		return nil, fmt.Errorf("plugin %q does not provide actions", name)
	}
	m.clients[name] = client
	m.providers[name] = provider
	return provider, nil
}

// Invoke implements runtime.ActionInvoker. Actions are addressed as
// "<plugin>/<verb>"; the plugin base name selects the executable.
func (m *Manager) Invoke(_ context.Context, action string, inputs map[string]string) (map[string]string, string, error) {
	name, _, ok := strings.Cut(action, "/")
	if !ok || name == "" {
		return nil, "", fmt.Errorf("invalid action %q: want <plugin>/<verb>", action)
	}
	m.mu.Lock()
	provider, err := m.ensure(name)
	m.mu.Unlock()
	if err != nil {
		return nil, "", err
	}
	res, err := provider.Invoke(action, inputs)
	if err != nil {
		return nil, "", err
	}
	return res.Outputs, res.Stdout, nil
}

// Describe returns metadata for a discovered plugin, launching it if needed.
func (m *Manager) Describe(name string) (pkgplugin.DescribeResult, error) {
	m.mu.Lock()
	provider, err := m.ensure(name)
	m.mu.Unlock()
	if err != nil {
		return pkgplugin.DescribeResult{}, err
	}
	return provider.Describe(), nil
}

// Close shuts down all launched plugin subprocesses.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.clients {
		c.Kill()
	}
	m.clients = map[string]*hplugin.Client{}
	m.providers = map[string]pkgplugin.ActionProvider{}
}
