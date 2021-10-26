package geodns

import (
	"fmt"
	"math"
	"net"
	"sort"
	"strings"

	"github.com/golang/geo/s2"
	"github.com/miekg/dns"
	"github.com/oschwald/geoip2-golang"
)

const maxDistance float64 = 360

var emptyLocation geoip2.City

type serverInfo struct {
	index        int
	distanceInfo *DistanceInfo
}

type recordInfo struct {
	endpoint string
	record   dns.RR
}

func (r *recordInfo) String() string {
	return r.record.String()
}

// ResponseFilter is a type of ResponseWriter that captures all messages written to it.
type ResponseFilter struct {
	dns.ResponseWriter
	filter *filter
	client net.IP
}

// NewResponseFilter makes and returns a new response filter.
func NewResponseFilter(w dns.ResponseWriter, filter *filter, client net.IP) *ResponseFilter {
	return &ResponseFilter{
		ResponseWriter: w,
		filter:         filter,
		client:         client,
	}
}

// WriteMsg records the message and its length written to it and call the
// underlying ResponseWriter's WriteMsg method.
func (r *ResponseFilter) WriteMsg(res *dns.Msg) error {
	qtype := res.Question[0].Qtype
	qName := res.Question[0].Name

	if !isSupportedType(qtype) {
		log.Debugf("unsupported type %s, nothing to do", dns.Type(qtype))
		return r.ResponseWriter.WriteMsg(res)
	}

	if len(res.Answer) == 0 {
		log.Debugf("answer is empty, nothing to do")
		return r.ResponseWriter.WriteMsg(res)
	}

	clientInf := r.filter.db.IPInfo(r.client)
	if clientInf.IsEmpty() {
		log.Warningf(formErrMessage(r.client))
		if r.filter.maxRecords < len(res.Answer) {
			res.Answer = res.Answer[:r.filter.maxRecords]
		}
		return r.ResponseWriter.WriteMsg(res)
	}

	healthy := r.filterHealthy(qName, res.Answer)
	if len(healthy) == 0 {
		log.Warningf("no answer returned: couldn't resolve %s: no healthy IPs", qName)
	}
	distances := make([]serverInfo, len(healthy))

	for i, rec := range healthy {
		serverInf := r.filter.db.IPInfo(net.ParseIP(rec.endpoint))
		if serverInf.IsEmpty() {
			log.Debugf(formErrMessage(rec))
			distances[i] = serverInfo{index: i, distanceInfo: &DistanceInfo{Distance: maxDistance}}
			continue
		}

		dist := distance(clientInf, serverInf)
		distances[i] = serverInfo{index: i, distanceInfo: dist}
	}

	res.Answer = chooseClosest(healthy, distances, r.filter.maxRecords)
	return r.ResponseWriter.WriteMsg(res)
}

func (r *ResponseFilter) filterHealthy(qName string, records []dns.RR) []*recordInfo {
	results := make([]*recordInfo, 0, len(records))
	endpoints := make([]string, len(records))

	for i, ans := range records {
		split := strings.Split(ans.String(), "\t")
		endpoints[i] = split[len(split)-1]
	}

	for i, healthy := range r.filter.health.check(endpoints) {
		if healthy {
			results = append(results, &recordInfo{record: records[i], endpoint: endpoints[i]})
		} else {
			log.Warningf("tried to resolve the %s, healthcheck %s failed",
				qName, net.JoinHostPort(endpoints[i], r.filter.health.port))
		}
	}

	return results
}

// Write is a wrapper that records the length of the messages that get written to it.
func (r *ResponseFilter) Write(buf []byte) (int, error) {
	return r.ResponseWriter.Write(buf)
}

func isSupportedType(qtype uint16) bool {
	return qtype == dns.TypeA || qtype == dns.TypeAAAA
}

func distance(from, to *IPInformation) *DistanceInfo {
	res := &DistanceInfo{Distance: maxDistance}
	if from == nil || to == nil {
		return res
	}

	var fromCountry, toCountry uint
	fromLocation, toLocation := emptyLocation.Location, emptyLocation.Location
	if from.City != nil {
		fromCountry = from.City.Country.GeoNameID
		fromLocation = from.City.Location
	}
	if to.City != nil {
		toCountry = to.City.Country.GeoNameID
		toLocation = to.City.Location
	}

	if fromLocation != emptyLocation.Location && toLocation != emptyLocation.Location {
		ll1 := s2.LatLngFromDegrees(fromLocation.Latitude, fromLocation.Longitude)
		ll2 := s2.LatLngFromDegrees(toLocation.Latitude, toLocation.Longitude)
		angle := ll1.Distance(ll2)
		res.Distance = math.Abs(angle.Degrees())
	}

	if fromCountry == 0 && from.Country != nil {
		fromCountry = from.Country.Country.GeoNameID
	}
	if toCountry == 0 && to.Country != nil {
		toCountry = to.Country.Country.GeoNameID
	}
	res.CountryMatched = fromCountry == toCountry

	return res
}

func chooseClosest(records []*recordInfo, distance []serverInfo, max int) []dns.RR {
	if len(records) < max {
		max = len(records)
	}

	sort.Slice(distance, func(i, j int) bool {
		di1 := distance[i].distanceInfo
		di2 := distance[j].distanceInfo

		if di1.Distance == maxDistance && di2.Distance == maxDistance {
			return di1.CountryMatched
		}

		return di1.Distance < di2.Distance
	})

	results := make([]dns.RR, max)
	for i := 0; i < max; i++ {
		results[i] = records[distance[i].index].record
	}

	return results
}

func formErrMessage(data fmt.Stringer) string {
	return fmt.Sprintf("couldn't get location %s from db: not found", data)
}
