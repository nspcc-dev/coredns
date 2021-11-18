package nns

import (
	"context"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"
	"github.com/miekg/dns"
	"github.com/nspcc-dev/neo-go/pkg/rpc/client"
	"github.com/stretchr/testify/require"
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
	require.Equal(t, "0e99bef139732856362899310a9bac1211f72d06", res)
}

func TestMapping(t *testing.T) {
	for _, tc := range []struct {
		dnsDomain string
		nnsDomain string
		request   string
		expected  string
	}{
		{
			dnsDomain: ".",
			nnsDomain: "",
			request:   "test.neofs",
			expected:  "test.neofs",
		},
		{
			dnsDomain: ".",
			nnsDomain: "",
			request:   "test.neofs.",
			expected:  "test.neofs",
		},
		{
			dnsDomain: ".",
			nnsDomain: "container.",
			request:   "test.neofs",
			expected:  "test.neofs.container",
		},
		{
			dnsDomain: ".",
			nnsDomain: ".container",
			request:   "test.neofs.",
			expected:  "test.neofs.container",
		},
		{
			dnsDomain: "containers.testnet.fs.neo.org.",
			nnsDomain: "container",
			request:   "containers.testnet.fs.neo.org",
			expected:  "container",
		},
		{
			dnsDomain: ".containers.testnet.fs.neo.org",
			nnsDomain: "container",
			request:   "containers.testnet.fs.neo.org.",
			expected:  "container",
		},
		{
			dnsDomain: "containers.testnet.fs.neo.org.",
			nnsDomain: "container",
			request:   "nicename.containers.testnet.fs.neo.org",
			expected:  "nicename.container",
		},
	} {
		nns := &NNS{}
		nns.setDNSDomain(tc.dnsDomain)
		nns.setNNSDomain(tc.nnsDomain)

		res := nns.prepareName(tc.request)
		require.Equal(t, tc.expected, res)
	}
}
