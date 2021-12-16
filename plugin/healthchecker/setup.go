package healthchecker

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
)

func init() {
	plugin.Register(pluginName, setup)
}

func setup(c *caddy.Controller) error {
	c.Next()
	filter, err := filterParamsParse(c)
	if err != nil {
		return err
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		return HealthChecker{
			Next:   next,
			filter: filter,
		}
	})

	return nil
}

func filterParamsParse(c *caddy.Controller) (*HealthCheckFilter, error) {
	var checker Checker
	var err error
	args := c.RemainingArgs()
	if len(args) < 4 {
		return nil, plugin.Error(pluginName,
			fmt.Errorf("the following format is supported: HEALTHCHECK_METHOD CACHE_SIZE "+
				"HEALTHCHECK_INTERVAL_IN_MS ORIGIN. [NAME_FILTER1 NAME_FILTER2]"))
	}

	if strings.Contains(args[0], "http") {
		checker, err = NewHttpChecker(args[0])
		if err != nil {
			return nil, err
		}
	}
	//parsing cache size
	size, err := strconv.Atoi(args[1])
	if err != nil || size <= 0 {
		return nil, plugin.Error(pluginName, fmt.Errorf("invalid cache size: %s", args[1]))
	}

	// parsing check interval
	interval, err := time.ParseDuration(args[2])
	if err != nil || interval <= 0 {
		return nil, plugin.Error(pluginName, fmt.Errorf("invalid endpoint check interval: %s", args[2]))
	}

	// parsing origin
	origin := args[3]
	if !strings.HasSuffix(origin, ".") {
		return nil, plugin.Error(pluginName, fmt.Errorf("invalid or missing origin: %s, the value must end in a dot",
			args[3]))
	}

	// parsing names
	names := make(map[string]struct{})
	for i := 4; i < len(args); i++ {
		if args[i] == "@" {
			names[origin] = struct{}{}
		} else {
			names[args[i]+"."+origin] = struct{}{}
		}
	}

	return NewHealthCheckFilter(checker, size, interval, names)
}
