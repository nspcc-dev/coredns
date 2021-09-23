package nns

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	nns "github.com/nspcc-dev/neo-go/examples/nft-nd-nns"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/rpc/client"
)

type NNS struct {
	Next   plugin.Handler
	Client *client.Client
	CS     *state.Contract
	Log    clog.P
}

// ServeDNS implements the plugin.Handler interface.
// This method gets called when example is used in a Server.
func (n NNS) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	res, err := n.resolveRecord(request.Request{W: w, Req: r})
	if err != nil {
		n.Log.Warning(err)
		return plugin.NextOrFailure(n.Name(), n.Next, ctx, w, r)
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Answer = []dns.RR{res}

	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

// Name implements the Handler interface.
func (n NNS) Name() string { return pluginName }

func (n NNS) resolve(name string, nnsType nns.RecordType) (string, error) {
	name = strings.TrimSuffix(name, ".")
	return n.Client.NNSResolve(n.CS.Hash, name, nnsType)
}

func (n NNS) resolveRecord(state request.Request) (dns.RR, error) {
	nnsType, err := getNNSType(state)
	if err != nil {
		return nil, err
	}

	s, err := n.resolve(state.QName(), nnsType)
	if err != nil {
		return nil, err
	}

	return formResRecord(state, s)
}

func getNNSType(req request.Request) (nns.RecordType, error) {
	switch req.QType() {
	case dns.TypeTXT:
		return nns.TXT, nil
	case dns.TypeA:
		return nns.A, nil
	case dns.TypeAAAA:
		return nns.AAAA, nil
	case dns.TypeCNAME:
		return nns.CNAME, nil
	}
	return 0, fmt.Errorf("usupported record type: %s", dns.Type(req.QType()))
}

func formResRecord(req request.Request, res string) (dns.RR, error) {
	hdr := dns.RR_Header{Name: req.QName(), Rrtype: req.QType(), Class: req.QClass(), Ttl: 0}

	switch req.QType() {
	case dns.TypeTXT:
		return &dns.TXT{Hdr: hdr, Txt: []string{res}}, nil
	case dns.TypeA:
		return &dns.A{Hdr: hdr, A: net.ParseIP(res)}, nil
	case dns.TypeAAAA:
		return &dns.AAAA{Hdr: hdr, AAAA: net.ParseIP(res)}, nil
	case dns.TypeCNAME:
		return &dns.CNAME{Hdr: hdr, Target: res}, nil
	}

	return nil, fmt.Errorf("usupported record type: %s", dns.Type(req.QType()))
}
