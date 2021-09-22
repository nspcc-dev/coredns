package geodns

import (
	"context"
	"fmt"
	"net"
	"path/filepath"

	"github.com/coredns/coredns/plugin"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/miekg/dns"
	"github.com/oschwald/geoip2-golang"
)

var log = clog.NewWithPlugin(pluginName)

type GeoDNS struct {
	Next   plugin.Handler
	filter *filter
}

type filter struct {
	db         db
	maxRecords int
	health     *checker
}

// the following borrowed from geoip plugin
type db struct {
	*geoip2.Reader
	// provides defines the schemas that can be obtained by querying this database, by using
	// bitwise operations.
	provides int
}

const (
	city = 1 << iota
)

var probingIP = net.ParseIP("127.0.0.1")

func newGeoDNS(dbPath string, maxRecords int) (*GeoDNS, error) {
	reader, err := geoip2.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database file: %v", err)
	}
	db := db{Reader: reader}
	schemas := []struct {
		provides int
		name     string
		validate func() error
	}{
		{name: "city", provides: city, validate: func() error { _, err := reader.City(probingIP); return err }},
	}
	// Query the database to figure out the database type.
	for _, schema := range schemas {
		if err := schema.validate(); err != nil {
			// If we get an InvalidMethodError then we know this database does not provide that schema.
			if _, ok := err.(geoip2.InvalidMethodError); !ok {
				return nil, fmt.Errorf("unexpected failure looking up database %q schema %q: %v", filepath.Base(dbPath), schema.name, err)
			}
		} else {
			db.provides = db.provides | schema.provides
		}
	}

	if db.provides&city == 0 {
		return nil, fmt.Errorf("database does not provide city schema")
	}

	return &GeoDNS{
		filter: &filter{
			db:         db,
			maxRecords: maxRecords,
			health:     newHealthChecker(),
		},
	}, nil
}

// ServeDNS implements the plugin.Handler interface.
func (g GeoDNS) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	var realIP net.IP

	if addr, ok := w.RemoteAddr().(*net.UDPAddr); ok {
		realIP = make(net.IP, len(addr.IP))
		copy(realIP, addr.IP)
	} else if addr, ok := w.RemoteAddr().(*net.TCPAddr); ok {
		realIP = make(net.IP, len(addr.IP))
		copy(realIP, addr.IP)
	}

	var ip net.IP // EDNS CLIENT SUBNET or real IP
	if option := r.IsEdns0(); option != nil {
		for _, s := range option.Option {
			switch e := s.(type) {
			case *dns.EDNS0_SUBNET:
				log.Debug("Got edns-client-subnet", e.Address, e.Family, e.SourceNetmask, e.SourceScope)
				if e.Address != nil {
					ip = e.Address
				}
			}
		}
	}

	if len(ip) == 0 { // no edns client subnet
		ip = realIP
	}

	rw := NewResponseFilter(w, g.filter, ip)
	return plugin.NextOrFailure(pluginName, g.Next, ctx, rw, r)
}

// Name implements the Handler interface.
func (g GeoDNS) Name() string { return pluginName }
