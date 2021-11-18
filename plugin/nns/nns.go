package nns

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/plugin/transfer"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	nns "github.com/nspcc-dev/neo-go/examples/nft-nd-nns"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/rpc/client"
)

type NNS struct {
	Next      plugin.Handler
	Client    *client.Client
	CS        *state.Contract
	Log       clog.P
	nnsDomain string
	dnsDomain string
}

const dot = "."

// ServeDNS implements the plugin.Handler interface.
// This method gets called when example is used in a Server.
func (n NNS) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	res, err := n.resolveRecords(request.Request{W: w, Req: r})
	if err != nil {
		n.Log.Warning(err)
		return plugin.NextOrFailure(n.Name(), n.Next, ctx, w, r)
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Answer = res

	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

// Name implements the Handler interface.
func (n NNS) Name() string { return pluginName }

// Transfer implements the transfer.Transfer interface.
func (n NNS) Transfer(zone string, serial uint32) (<-chan []dns.RR, error) {
	trimmedZone := n.prepareName(zone)
	records, err := getRecords(n.Client, n.CS.Hash, trimmedZone, nns.RecordType(dns.TypeSOA))
	if err != nil {
		n.Log.Warningf("couldn't transfer zone '%s' as '%s': %s", zone, trimmedZone, err.Error())
		return nil, transfer.ErrNotAuthoritative
	}
	if len(records) == 0 {
		return nil, transfer.ErrNotAuthoritative
	}

	ch := make(chan []dns.RR)
	go func() {
		defer close(ch)

		recs, err := n.zoneTransfer(trimmedZone)
		if err != nil {
			n.Log.Warningf("couldn't transfer zone '%s' as '%s' : %s", zone, trimmedZone, err.Error())
			return
		}

		ch <- recs
	}()

	return ch, nil
}

func (n *NNS) setDNSDomain(name string) {
	n.dnsDomain = strings.Trim(name, dot)
}

func (n *NNS) setNNSDomain(name string) {
	n.nnsDomain = strings.Trim(name, dot)
}

func (n NNS) prepareName(name string) string {
	name = strings.TrimSuffix(name, dot)
	if n.nnsDomain != "" {
		name = strings.TrimSuffix(strings.TrimSuffix(name, n.dnsDomain), dot)
		if name != "" {
			name += dot
		}
		name += n.nnsDomain
	}
	return name
}

func (n NNS) resolveRecords(state request.Request) ([]dns.RR, error) {
	name := n.prepareName(state.QName())

	nnsType, err := getNNSType(state)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve '%s' (type %d) as '%s': %w", state.QName(), state.QType(), name, err)
	}

	resolved, err := resolve(n.Client, n.CS.Hash, name, nnsType)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve '%s' (type %d) as '%s': %w", state.QName(), state.QType(), name, err)
	}

	hdr := dns.RR_Header{Name: state.QName(), Rrtype: state.QType(), Class: state.QClass(), Ttl: 0}
	res, err := formResRecords(hdr, resolved)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve '%s' (type %d) as '%s': %w", state.QName(), state.QType(), name, err)
	}
	return res, nil
}

func (n NNS) zoneTransfer(name string) ([]dns.RR, error) {
	records, err := getAllRecords(n.Client, n.CS.Hash, name)
	if err != nil {
		return nil, err
	}

	numSoa, index := 0, -1
	for i, record := range records {
		records[i].Name = appendRoot(record.Name)
		if record.Type == nns.RecordType(dns.TypeSOA) {
			numSoa++
			index = i
		}
	}
	if numSoa != 1 {
		return nil, fmt.Errorf("invalid number of soa records: %d", numSoa)
	}

	if index != 0 {
		records[0], records[index] = records[index], records[0]
	}
	return formZoneTransfer(records)
}

func formZoneTransfer(records []nnsRecord) ([]dns.RR, error) {
	if len(records) == 0 {
		return nil, fmt.Errorf("records must not be empty")
	}
	soaRecord, err := formSoaRecord(records[0])
	if err != nil {
		return nil, err
	}

	results := make([]dns.RR, 1, len(records)+1)
	results[0] = soaRecord
	for _, record := range records[1:] {
		rec, err := formRec(uint16(record.Type), record.Data, dns.RR_Header{
			Name:   record.Name,
			Rrtype: uint16(record.Type),
			Class:  dns.ClassINET,
		})
		if err != nil {
			return nil, err
		}
		results = append(results, rec)
	}
	results = append(results, soaRecord)

	return results, nil
}

func formSoaRecord(rec nnsRecord) (*dns.SOA, error) {
	if rec.Type != nns.RecordType(dns.TypeSOA) {
		return nil, fmt.Errorf("invalid type for soa record")
	}
	split := strings.Split(rec.Data, " ")
	if len(split) != 7 {
		return nil, fmt.Errorf("invalid soa record: %s", rec.Data)
	}

	name := appendRoot(split[0])
	if rec.Name != name {
		return nil, fmt.Errorf("invalid soa record, mismatched names: %s %s", rec.Name, name)
	}

	lenSerial := len(split[2])
	if lenSerial > 10 { // timestamp with second precision
		lenSerial = 10
	}
	serial, err := parseUint32(split[2][:lenSerial])
	if err != nil {
		return nil, fmt.Errorf("invalid soa record, invalid serial: %s", split[2])
	}
	refresh, err := parseUint32(split[3])
	if err != nil {
		return nil, fmt.Errorf("invalid soa record, invalid refresh: %s", split[3])
	}
	retry, err := parseUint32(split[4])
	if err != nil {
		return nil, fmt.Errorf("invalid soa record, invalid retry: %s", split[4])
	}
	expire, err := parseUint32(split[5])
	if err != nil {
		return nil, fmt.Errorf("invalid soa record, invalid expire: %s", split[5])
	}
	ttl, err := parseUint32(split[6])
	if err != nil {
		return nil, fmt.Errorf("invalid soa record, invalid ttl: %s", split[6])
	}

	return &dns.SOA{
		Hdr: dns.RR_Header{
			Name:   name,
			Rrtype: dns.TypeSOA,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Ns:      name,
		Mbox:    strings.ReplaceAll(appendRoot(split[1]), "@", "."),
		Serial:  serial,
		Refresh: refresh,
		Retry:   retry,
		Expire:  expire,
		Minttl:  ttl,
	}, nil
}

func appendRoot(data string) string {
	if len(data) > 0 && data[len(data)-1] != '.' {
		return data + "."
	}
	return data
}

func parseUint32(data string) (uint32, error) {
	parsed, err := strconv.ParseUint(data, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(parsed), nil
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

func formResRecords(hdr dns.RR_Header, resolved []string) ([]dns.RR, error) {
	var records []dns.RR
	for _, record := range resolved {
		rec, err := formRec(hdr.Rrtype, record, hdr)
		if err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, nil
}

func formRec(reqType uint16, res string, hdr dns.RR_Header) (dns.RR, error) {
	switch reqType {
	case dns.TypeTXT:
		return &dns.TXT{Hdr: hdr, Txt: []string{res}}, nil
	case dns.TypeA:
		return &dns.A{Hdr: hdr, A: net.ParseIP(res)}, nil
	case dns.TypeAAAA:
		return &dns.AAAA{Hdr: hdr, AAAA: net.ParseIP(res)}, nil
	case dns.TypeCNAME:
		return &dns.CNAME{Hdr: hdr, Target: res}, nil
	}

	return nil, fmt.Errorf("usupported record type: %s", dns.Type(reqType))
}
