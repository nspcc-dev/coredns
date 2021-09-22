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
	index    int
	distance float64
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

	if !isSupportedType(qtype) {
		log.Debugf("unsupported type %s, nothing to do", dns.Type(qtype))
		return r.ResponseWriter.WriteMsg(res)
	}

	clientLocation, err := r.filter.db.City(r.client)
	if err != nil || clientLocation.Location == emptyLocation.Location {
		log.Warningf(formErrMessage(r.client, err))
		if r.filter.maxRecords < len(res.Answer) {
			res.Answer = res.Answer[:r.filter.maxRecords]
		}
		return r.ResponseWriter.WriteMsg(res)
	}

	healthy := r.filterHealthy(res.Answer)
	distances := make([]serverInfo, len(healthy))

	for i, rec := range healthy {
		serverLocation, err := r.filter.db.City(net.ParseIP(rec.endpoint))
		if err != nil || serverLocation.Location == emptyLocation.Location {
			log.Debugf(formErrMessage(rec, err))
			distances[i] = serverInfo{index: i, distance: maxDistance}
			continue
		}

		dist := distance(clientLocation, serverLocation)
		distances[i] = serverInfo{index: i, distance: dist}
	}

	res.Answer = chooseClosest(healthy, distances, r.filter.maxRecords)
	return r.ResponseWriter.WriteMsg(res)
}

func (r *ResponseFilter) filterHealthy(records []dns.RR) []*recordInfo {
	results := make([]*recordInfo, 0, len(records))
	endpoints := make([]string, len(records))

	for i, ans := range records {
		split := strings.Split(ans.String(), "\t")
		endpoints[i] = split[len(split)-1]
	}

	for i, healthy := range r.filter.health.check(endpoints) {
		if healthy {
			results = append(results, &recordInfo{record: records[i], endpoint: endpoints[i]})
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

func distance(from, to *geoip2.City) float64 {
	if from == nil || to == nil {
		return maxDistance
	}
	ll1 := s2.LatLngFromDegrees(from.Location.Latitude, from.Location.Longitude)
	ll2 := s2.LatLngFromDegrees(to.Location.Latitude, to.Location.Longitude)
	angle := ll1.Distance(ll2)
	return math.Abs(angle.Degrees())
}

func chooseClosest(records []*recordInfo, distance []serverInfo, max int) []dns.RR {
	if len(records) < max {
		max = len(records)
	}

	sort.Slice(distance, func(i, j int) bool {
		return distance[i].distance < distance[j].distance
	})

	results := make([]dns.RR, max)
	for i := 0; i < max; i++ {
		results[i] = records[distance[i].index].record
	}

	return results
}

func formErrMessage(data fmt.Stringer, err error) string {
	if err != nil {
		return fmt.Sprintf("couldn't get location %s from db: %s", data, err)
	}
	return fmt.Sprintf("couldn't get location %s from db: not found", data)
}
