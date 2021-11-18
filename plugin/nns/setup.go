package nns

import (
	"context"
	"fmt"
	"net/url"

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
	URL, err := url.Parse(c.Key)
	if err != nil {
		return plugin.Error(pluginName, c.Err(err.Error()))
	}

	endpoint, nnsDomain, err := parseArgs(c)
	if err != nil {
		return err
	}

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
		nns := &NNS{
			Next:   next,
			Client: cli,
			CS:     cs,
			Log:    clog.NewWithPlugin(pluginName),
		}
		nns.setNNSDomain(nnsDomain)
		nns.setDNSDomain(URL.Hostname())

		return *nns
	})

	return nil
}

func parseArgs(c *caddy.Controller) (string, string, error) {
	c.Next()
	args := c.RemainingArgs()
	if len(args) < 1 || len(args) > 2 {
		return "", "", plugin.Error(pluginName, fmt.Errorf("support the following args template: 'NEO_CHAIN_ENDPOINT [NNS_DOMAIN]'"))
	}
	endpoint := args[0]
	if URL, err := url.Parse(endpoint); err != nil {
		return "", "", plugin.Error(pluginName, fmt.Errorf("couldn't parse endpoint: %w", err))
	} else if URL.Scheme == "" || URL.Port() == "" {
		return "", "", plugin.Error(pluginName, fmt.Errorf("invalid endpoint: %s", endpoint))
	}

	nnsDomain := ""
	if len(args) == 2 {
		nnsDomain = args[1]
	}

	return endpoint, nnsDomain, nil
}
