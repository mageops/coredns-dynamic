package dynamic

import (
	"time"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
)

type Settings struct {
	Secret      string
	Addr        string
	Zones       []string
	HostTimeout time.Duration
}

var (
	Secret string
	log    = clog.NewWithPlugin("dynamic")
)

func init() {
	plugin.Register("dynamic", setup)
}

// Parse config
func setup(c *caddy.Controller) error {

	settings, err := parse(c)

	if err != nil {
		return err
	}

	dynamic := New(settings)

	c.OnStartup(dynamic.OnStartup)
	c.OnRestart(dynamic.OnRestart)
	c.OnFinalShutdown(dynamic.OnFinalShutdown)

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		dynamic.Next = next
		return dynamic
	})

	return nil
}

func parse(c *caddy.Controller) (*Settings, error) {
	zones := plugin.OriginsFromArgsOrServerBlock(c.ServerBlockKeys, []string{"."})
	settings := &Settings{
		Zones:       zones,
		HostTimeout: time.Minute * 5,
	}

	c.RemainingArgs()
	for c.NextBlock() {
		switch c.Val() {
		case "secret":
			args := c.RemainingArgs()
			if len(args) > 1 {
				return nil, c.ArgErr()
			}
			settings.Secret = args[0]
			continue
		case "addr":
			args := c.RemainingArgs()
			if len(args) > 1 {
				return nil, c.ArgErr()
			}
			settings.Addr = args[0]
			continue
		case "host_timeout":
			args := c.RemainingArgs()
			if len(args) != 1 {
				return nil, c.ArgErr()
			}
			timeout, err := time.ParseDuration(args[0])
			if err != nil {
				return nil, err
			}
			settings.HostTimeout = timeout
			continue
		default:
			return nil, c.Errf("unknown property '%s'", c.Val())
		}
	}

	return settings, nil
}
