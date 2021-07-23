package dynamic

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/plugin/pkg/upstream"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
)

type Dynamic struct {
	Next                plugin.Handler
	Upstream            *upstream.Upstream
	addr                string
	secret              string
	keepBackendDuration time.Duration
	zones               []string

	ln      net.Listener
	lnSetup bool

	mux *http.ServeMux
	srv *http.Server

	backends         map[string]*BackendInfo
	lastAddrChanged  time.Time
	nextCleanupAfter *time.Time
}

func (d Dynamic) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	qname := state.QName()
	zone := plugin.Zones(d.zones).Matches(qname)

	if zone == "" {
		return plugin.NextOrFailure(d.Name(), d.Next, ctx, w, r)
	}
	zone = qname[len(qname)-len(zone):] // maintain case of original query
	state.Zone = zone

	qBackend, err := dnsutil.TrimZone(qname, zone)
	if err != nil {
		return dns.RcodeServerFailure, err
	}

	var records []dns.RR

	switch state.QType() {
	case dns.TypeA:
		now := time.Now()
		addrInfo, ok := d.backends[qBackend]
		if !ok {
			return dns.RcodeBadName, nil
		}

		for addr, info := range addrInfo.addresses {
			addr := net.ParseIP(addr)
			if addr.To4() == nil {
				continue
			}
			ttl := info.validUntil.Sub(now)

			if ttl.Seconds() <= 0 {
				continue
			}

			records = append(records, &dns.A{
				Hdr: dns.RR_Header{
					Name:   qname,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    uint32(ttl.Seconds()),
				},
				A: addr,
			})
		}
	}

	if len(records) == 0 {
		return dns.RcodeBadName, nil
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true
	m.Answer = append(m.Answer, records...)
	w.WriteMsg(m)

	d.maybeCleanup()
	return dns.RcodeSuccess, nil
}

func New(settings *Settings) *Dynamic {
	d := &Dynamic{
		Upstream:            upstream.New(),
		addr:                settings.Addr,
		secret:              settings.Secret,
		zones:               settings.Zones,
		keepBackendDuration: settings.HostTimeout,

		backends: make(map[string]*BackendInfo),
	}

	return d
}

func (d Dynamic) Name() string { return "dynamic" }
