package nns

import (
	"context"
	"fmt"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/nspcc-dev/neo-go/pkg/rpc/client"
)

const pluginName = "nns"

func init() {
	plugin.Register(pluginName, setup)
}

func setup(c *caddy.Controller) error {
	c.Next()
	args := c.RemainingArgs()
	if len(args) != 1 {
		return plugin.Error(pluginName, fmt.Errorf("exactly one arg is expected (morph chain endpoint)"))
	}
	endpoint := args[0]

	cli, err := client.New(context.TODO(), endpoint, client.Options{})
	if err != nil {
		return plugin.Error(pluginName, c.Err(err.Error()))
	}
	if err := cli.Init(); err != nil {
		return plugin.Error(pluginName, c.Err(err.Error()))
	}

	cs, err := cli.GetContractStateByID(1)
	if err != nil {
		return plugin.Error(pluginName, c.Err(err.Error()))
	}

	// Add the Plugin to CoreDNS, so Servers can use it in their plugin chain.
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		return NNS{
			Next:   next,
			Client: cli,
			CS:     cs,
			Log:    clog.NewWithPlugin(pluginName),
		}
	})

	return nil
}
