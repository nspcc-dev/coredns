package nns

import (
	"context"
	"fmt"
	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	nns "github.com/nspcc-dev/neo-go/examples/nft-nd-nns"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/rpc/client"
	"strings"
)

type NNS struct {
	Next   plugin.Handler
	Client *client.Client
	CS     *state.Contract
	Log    clog.P
}

// ServeDNS implements the plugin.Handler interface. This method gets called when example is used
// in a Server.
func (n NNS) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	s, err := n.resolve(state.QName(), state.QType())
	if err != nil {
		n.Log.Warning(err)
		return plugin.NextOrFailure(n.Name(), n.Next, ctx, w, r)
	}

	m := new(dns.Msg)
	m.SetReply(r)
	hdr := dns.RR_Header{Name: state.QName(), Rrtype: dns.TypeTXT, Class: state.QClass(), Ttl: 0}
	m.Answer = []dns.RR{&dns.TXT{Hdr: hdr, Txt: []string{s}}}

	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

// Name implements the Handler interface.
func (n NNS) Name() string { return pluginName }

func (n NNS) resolve(name string, dnsType uint16) (string, error) {
	var nnsType nns.RecordType
	switch dnsType {
	case dns.TypeTXT:
		nnsType = nns.TXT
	case dns.TypeA:
		nnsType = nns.A
	case dns.TypeAAAA:
		nnsType = nns.AAAA
	case dns.TypeCNAME:
		nnsType = nns.CNAME
	default:
		return "", fmt.Errorf("usupported record type: %s", dns.Type(dnsType))
	}

	name = strings.TrimSuffix(name, ".")
	return n.Client.NNSResolve(n.CS.Hash, name, nnsType)
}
