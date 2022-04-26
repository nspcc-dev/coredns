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
	"github.com/nspcc-dev/neo-go/pkg/util"
)

const pluginName = "nns"

type Params struct {
	Endpoint     string
	ContractHash util.Uint160
	Domain       string
}

func init() {
	plugin.Register(pluginName, setup)
}

func setup(c *caddy.Controller) error {
	URL, err := url.Parse(c.Key)
	if err != nil {
		return plugin.Error(pluginName, c.Err(err.Error()))
	}

	args, err := parseArgs(c)
	if err != nil {
		return err
	}

	cli, err := client.New(context.TODO(), args.Endpoint, client.Options{})
	if err != nil {
		return plugin.Error(pluginName, c.Err(err.Error()))
	}
	if err := cli.Init(); err != nil {
		return plugin.Error(pluginName, c.Err(err.Error()))
	}

	if args.ContractHash.Equals(util.Uint160{}) {
		cs, err := cli.GetContractStateByID(1)
		if err != nil {
			return plugin.Error(pluginName, c.Err(err.Error()))
		}
		args.ContractHash = cs.Hash
	} else {
		_, err := cli.GetContractStateByHash(args.ContractHash)
		if err != nil {
			return plugin.Error(pluginName, c.Err(err.Error()))
		}
	}

	// Add the Plugin to CoreDNS, so Servers can use it in their plugin chain.
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		nns := &NNS{
			Next:         next,
			Client:       cli,
			ContractHash: args.ContractHash,
			Log:          clog.NewWithPlugin(pluginName),
		}
		nns.setNNSDomain(args.Domain)
		nns.setDNSDomain(URL.Hostname())

		return *nns
	})

	return nil
}

func parseArgs(c *caddy.Controller) (*Params, error) {
	c.Next()
	args := c.RemainingArgs()
	var (
		err error
		res Params
	)

	if len(args) < 2 || len(args) > 3 {
		return nil, plugin.Error(pluginName, fmt.Errorf("support the following args template: 'NEO_CHAIN_ENDPOINT CONTRACT_ADDRESS [NNS_DOMAIN]'"))
	}

	res.Endpoint = args[0]
	if URL, err := url.Parse(res.Endpoint); err != nil {
		return nil, plugin.Error(pluginName, fmt.Errorf("couldn't parse endpoint: %w", err))
	} else if URL.Scheme == "" || URL.Port() == "" {
		return nil, plugin.Error(pluginName, fmt.Errorf("invalid endpoint: %s", res.Endpoint))
	}

	hexStr := args[1]
	if hexStr != "-" {
		res.ContractHash, err = util.Uint160DecodeStringLE(hexStr)
		if err != nil {
			return nil, plugin.Error(pluginName, fmt.Errorf("invalid nns contract address"))
		}
	}

	if len(args) == 3 {
		res.Domain = args[2]
	}

	return &res, nil
}
