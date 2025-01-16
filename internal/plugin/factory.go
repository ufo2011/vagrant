// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package plugin

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/hashicorp/go-argmapper"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"

	"github.com/hashicorp/vagrant-plugin-sdk/component"
	"github.com/hashicorp/vagrant-plugin-sdk/internal-shared/cleanup"
	"github.com/hashicorp/vagrant-plugin-sdk/internal-shared/pluginclient"
)

// exePath contains the value of os.Executable. We cache the value because
// we use it a lot and subsequent calls perform syscalls.
var exePath string

func init() {
	var err error
	exePath, err = os.Executable()
	if err != nil {
		panic(err)
	}
}

func Factory(
	cmd *exec.Cmd, // Plugin command to run
) PluginRegistration {
	return func(log hclog.Logger) (p *Plugin, err error) {
		// We have to copy the command because go-plugin will set some
		// fields on it.
		cmdCopy := *cmd

		log = log.Named("factory")

		nlog := log.ResetNamed("vagrant.plugin")
		config := pluginclient.ClientConfig(nlog)
		config.Cmd = &cmdCopy
		config.Logger = nlog

		// Log that we're going to launch this
		log.Info("launching plugin",
			"path", cmd.Path,
			"args", cmd.Args)

		// Connect to the plugin
		client := plugin.NewClient(config)

		// If we encounter any errors during setup, automatically
		// kill the client
		defer func() {
			if err != nil {
				client.Kill()
			}
		}()

		rpcClient, err := client.Client()
		if err != nil {
			log.Error("error creating plugin client",
				"error", err)

			return
		}

		raw, err := rpcClient.Dispense("plugininfo")
		if err != nil {
			log.Error("error requesting plugin information interface",
				"plugin", cmd.Path,
				"error", err)

			return
		}

		info, ok := raw.(component.PluginInfo)
		if !ok {
			log.Error("cannot load plugin info component",
				"plugin", cmd.Path)

			return nil, fmt.Errorf("failed to load plugin information interface")
		}

		mappers, err := pluginclient.Mappers(client)
		if err != nil {
			log.Error("error requesting plugin mappers",
				"error", err,
			)
			client.Kill()
			return nil, err
		}

		log.Info("collected mappers from plugin",
			"name", info.Name(),
			"mappers", mappers,
		)

		log.Info("plugin components and options",
			"components", info.ComponentTypes(),
			"options", info.ComponentOptions(),
		)

		p = &Plugin{
			Builtin:  false,
			Client:   rpcClient,
			Location: cmd.Path,
			Name:     info.Name(),
			Types:    info.ComponentTypes(),
			Options:  info.ComponentOptions(),
			Mappers:  mappers,
			cleaner:  cleanup.New(),
			logger:   nlog.Named(info.Name()),
			src:      client,
		}

		// Close the rpcClient when plugin is closed
		p.Closer(func() error {
			return rpcClient.Close()
		})

		return
	}
}

// BuiltinFactory creates a factory for a built-in plugin type.
func BuiltinFactory(name string) PluginRegistration {
	cmd := exec.Command(exePath, "plugin-run", name)

	// For non-windows systems, we attach stdout/stderr as extra fds
	// so that we can get direct access to the TTY if possible for output.
	if runtime.GOOS != "windows" {
		cmd.ExtraFiles = []*os.File{os.Stdout, os.Stderr}
	}

	return Factory(cmd)
}

func RubyFactory(
	rubyClient plugin.ClientProtocol,
	name string,
	typ component.Type,
	optsProto interface{},
) PluginRegistration {
	return func(log hclog.Logger) (*Plugin, error) {
		options, err := component.UnmarshalOptionsProto(typ, optsProto)
		if err != nil {
			return nil, err
		}
		return &Plugin{
			Builtin:  false,
			Client:   rubyClient,
			Location: "ruby-runtime",
			Name:     name,
			Types:    []component.Type{typ},
			Options:  map[component.Type]interface{}{typ: options},
			cleaner:  cleanup.New(),
			logger: log.ResetNamed(
				fmt.Sprintf("vagrant.legacy-plugin.%s.%s", strings.ToLower(typ.String()), name),
			),
		}, nil
	}
}

// Instance is the result generated by the factory. This lets us pack
// a bit more information into plugin-launched components.
type Instance struct {
	// Plugin name providing this component
	Name string

	// Type of component provided in this instance
	Type component.Type

	// Component is the dispensed component
	Component interface{}

	// Options for component type, see PluginInfo.ComponentOptions
	Options interface{}

	// Mappers is the list of mappers that this plugin is providing.
	Mappers []*argmapper.Func

	// The GRPCBroker attached to this plugin
	Broker *plugin.GRPCBroker

	// Parent component
	Parent *Instance

	// Closer is a function that should be called to clean up resources
	// associated with this plugin.
	Close func() error
}

func (i *Instance) Parents() []string {
	n := []string{}
	for p := i; p != nil; p = p.Parent {
		n = append(n, p.Name)
	}
	return n
}

func (i *Instance) ParentCount() int {
	return len(i.Parents())
}
