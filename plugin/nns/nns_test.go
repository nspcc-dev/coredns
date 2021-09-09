package nns

import (
	"context"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"
	"github.com/miekg/dns"
	"github.com/nspcc-dev/neo-go/pkg/rpc/client"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGetNetmapHash(t *testing.T) {
	ctx := context.Background()
	container := createDockerContainer(ctx, t, testImage)
	defer container.Terminate(ctx)

	cli, err := client.New(ctx, "http://localhost:30333", client.Options{})
	require.NoError(t, err)
	err = cli.Init()
	require.NoError(t, err)
	cs, err := cli.GetContractStateByID(1)
	require.NoError(t, err)

	nns := NNS{
		Next:   test.NextHandler(dns.RcodeSuccess, nil),
		Client: cli,
		CS:     cs,
	}

	req := new(dns.Msg)
	req.SetQuestion(dns.Fqdn("netmap.neofs"), dns.TypeTXT)
	req.Question[0].Qclass = dns.ClassCHAOS

	rec := dnstest.NewRecorder(&test.ResponseWriter{})
	status, err := nns.ServeDNS(context.TODO(), rec, req)
	require.NoError(t, err)
	require.Equal(t, dns.RcodeSuccess, status)

	res := rec.Msg.Answer[0].(*dns.TXT).Txt[0]
	require.Equal(t, "6b5a4b75ed4540d14420a7c3264bb936cb78a2d4", res)
}
