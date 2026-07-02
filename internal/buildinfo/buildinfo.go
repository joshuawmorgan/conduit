// Package buildinfo exposes version metadata stamped at build time via -ldflags.
package buildinfo

import (
	"fmt"
	"runtime"
)

// These variables are overridden at build time, e.g.:
//
//	go build -ldflags "-X github.com/conduit-io/conduit/internal/buildinfo.Version=v0.1.0"
var (
	Version   = "v0.0.0-dev"
	Commit    = "none"
	Date      = "unknown"
	BuiltBy   = "source"
	GoVersion = runtime.Version()
)

// FlowVersion is the FlowDSL language version implemented by this build.
const FlowVersion = "1.0"

// PluginProtocolVersion is the negotiated plugin ABI version.
const PluginProtocolVersion = 1

// String returns a one-line human-readable version string.
func String() string {
	return fmt.Sprintf("conduit %s (commit %s, built %s by %s, %s/%s, %s)",
		Version, Commit, Date, BuiltBy, runtime.GOOS, runtime.GOARCH, GoVersion)
}

// Map returns machine-readable version fields.
func Map() map[string]string {
	return map[string]string{
		"version":        Version,
		"commit":         Commit,
		"date":           Date,
		"builtBy":        BuiltBy,
		"go":             GoVersion,
		"os":             runtime.GOOS,
		"arch":           runtime.GOARCH,
		"flowVersion":    FlowVersion,
		"pluginProtocol": fmt.Sprintf("%d", PluginProtocolVersion),
	}
}
