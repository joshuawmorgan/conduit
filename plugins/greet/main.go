// Command conduit-plugin-greet is a sample Conduit plugin providing greeting
// actions. Build it and place the binary in a plugin dir (default
// .conduit/plugins) so `uses: "greet/hello"` resolves to it.
//
//	go build -o .conduit/plugins/conduit-plugin-greet ./plugins/greet
package main

import (
	"fmt"
	"strings"

	cplugin "github.com/conduit-io/conduit/pkg/plugin"
)

type provider struct{}

func (provider) Describe() cplugin.DescribeResult {
	return cplugin.DescribeResult{
		Name:    "greet",
		Version: "0.1.0",
		Actions: []string{"greet/hello", "greet/shout"},
	}
}

func (provider) Invoke(action string, inputs map[string]string) (cplugin.Result, error) {
	name := inputs["name"]
	if name == "" {
		name = "world"
	}
	switch action {
	case "greet/hello":
		msg := fmt.Sprintf("Hello, %s!", name)
		return cplugin.Result{Outputs: map[string]string{"greeting": msg}, Stdout: msg + "\n"}, nil
	case "greet/shout":
		msg := strings.ToUpper(fmt.Sprintf("HELLO, %s!!!", name))
		return cplugin.Result{Outputs: map[string]string{"greeting": msg}, Stdout: msg + "\n"}, nil
	default:
		return cplugin.Result{}, fmt.Errorf("unknown action %q", action)
	}
}

func main() {
	cplugin.Serve(provider{})
}
