// Package plugin defines the public Conduit plugin contract built on
// hashicorp/go-plugin. Plugins are separate executables that serve an
// ActionProvider over an RPC transport; the host dispenses and invokes them.
//
// A minimal plugin:
//
//	func main() {
//	    plugin.Serve(&plugin.ServeConfig{Plugins: map[string]hplugin.Plugin{
//	        "action": &plugin.ActionPlugin{Impl: myProvider{}},
//	    }})
//	}
package plugin

import (
	"net/rpc"

	hplugin "github.com/hashicorp/go-plugin"
)

// Handshake is the shared handshake used by host and plugins. A mismatch causes
// go-plugin to refuse the connection with a clear error (prevents running a
// plugin binary directly as a normal process).
var Handshake = hplugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "CONDUIT_PLUGIN",
	MagicCookieValue: "conduit-e2b7f1",
}

// Result is the output of an action invocation.
type Result struct {
	Outputs map[string]string
	Stdout  string
}

// InvokeArgs is the RPC request for an action invocation.
type InvokeArgs struct {
	Action string
	Inputs map[string]string
}

// DescribeResult lists the actions a plugin provides.
type DescribeResult struct {
	Name    string
	Version string
	Actions []string
}

// ActionProvider is the interface a plugin implements.
type ActionProvider interface {
	Describe() DescribeResult
	Invoke(action string, inputs map[string]string) (Result, error)
}

// ---- net/rpc glue ----

// ActionRPC is the host-side client implementing ActionProvider over RPC.
type ActionRPC struct{ client *rpc.Client }

func (a *ActionRPC) Describe() DescribeResult {
	var resp DescribeResult
	_ = a.client.Call("Plugin.Describe", struct{}{}, &resp)
	return resp
}

func (a *ActionRPC) Invoke(action string, inputs map[string]string) (Result, error) {
	var resp Result
	err := a.client.Call("Plugin.Invoke", InvokeArgs{Action: action, Inputs: inputs}, &resp)
	return resp, err
}

// ActionRPCServer is the plugin-side RPC server wrapping the implementation.
type ActionRPCServer struct{ Impl ActionProvider }

func (s *ActionRPCServer) Describe(_ struct{}, resp *DescribeResult) error {
	*resp = s.Impl.Describe()
	return nil
}

func (s *ActionRPCServer) Invoke(args InvokeArgs, resp *Result) error {
	r, err := s.Impl.Invoke(args.Action, args.Inputs)
	if err != nil {
		return err
	}
	*resp = r
	return nil
}

// ActionPlugin adapts an ActionProvider to go-plugin's Plugin interface.
type ActionPlugin struct {
	Impl ActionProvider
}

func (p *ActionPlugin) Server(*hplugin.MuxBroker) (any, error) {
	return &ActionRPCServer{Impl: p.Impl}, nil
}

func (p *ActionPlugin) Client(_ *hplugin.MuxBroker, c *rpc.Client) (any, error) {
	return &ActionRPC{client: c}, nil
}

// PluginSet is the dispensable plugin map shared by host and plugin.
func PluginSet() map[string]hplugin.Plugin {
	return map[string]hplugin.Plugin{"action": &ActionPlugin{}}
}

// ServeConfig re-exports the fields a plugin author needs.
type ServeConfig = hplugin.ServeConfig

// Serve starts a plugin. Convenience wrapper that injects the handshake.
func Serve(impl ActionProvider) {
	hplugin.Serve(&hplugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins:         map[string]hplugin.Plugin{"action": &ActionPlugin{Impl: impl}},
	})
}
