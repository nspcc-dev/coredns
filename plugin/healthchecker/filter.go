package healthchecker

import (
	"fmt"
	"regexp"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/miekg/dns"
	"go.uber.org/atomic"
)

type (
	HealthCheckFilter struct {
		cache    *lru.Cache
		checker  Checker
		interval time.Duration
		names    map[string]struct{}
		filters  []Filter
	}

	entry struct {
		dnsRecord dns.RR
		healthy   *atomic.Bool
		quit      chan struct{}
	}

	Filter interface {
		Match(string) bool
	}

	RegexpFilter struct {
		expr *regexp.Regexp
	}

	SimpleMatchFilter string
)

func (f SimpleMatchFilter) Match(rec string) bool {
	return string(f) == rec
}

func NewRegexpFilter(pattern string) (*RegexpFilter, error) {
	expr, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return &RegexpFilter{expr: expr}, nil
}

func (f *RegexpFilter) Match(rec string) bool {
	return f.expr.MatchString(rec)
}

func NewHealthCheckFilter(checker Checker, size int, interval time.Duration, filters []Filter) (*HealthCheckFilter, error) {
	if len(filters) == 0 {
		return nil, fmt.Errorf("filters must not be empty")
	}

	cache, err := lru.NewWithEvict(size, func(key interface{}, value interface{}) {
		if e, ok := value.(*entry); ok {
			close(e.quit)
		}
	})
	if err != nil {
		return nil, err
	}
	return &HealthCheckFilter{
		cache:    cache,
		checker:  checker,
		interval: interval,
		filters:  filters,
	}, nil
}

func (p *HealthCheckFilter) FilterRecords(records []dns.RR) []dns.RR {
	result := make([]dns.RR, 0, len(records))

	for _, r := range records {
		if matchFilters(p.filters, r.Header().Name) {
			e := p.get(r.String())
			if e != nil {
				if e.healthy.Load() {
					result = append(result, e.dnsRecord)
				}
				continue
			}
			p.put(r)
			log.Debugf("record '%s' will be cached", r.String())
		}
		result = append(result, r)
	}

	return result
}

func matchFilters(filters []Filter, record string) bool {
	for _, filter := range filters {
		if filter.Match(record) {
			return true
		}
	}

	return false
}

func (p *HealthCheckFilter) put(rec dns.RR) {
	health := p.checker.Check(rec)
	quit := make(chan struct{})
	record := &entry{
		dnsRecord: rec,
		healthy:   atomic.NewBool(health),
		quit:      quit,
	}
	p.cache.Add(rec.String(), record)

	ticker := time.NewTicker(p.interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-quit:
				return
			case <-ticker.C:
				e, ok := p.cache.Peek(rec.String())
				if !ok {
					return
				}
				val, ok := e.(*entry)
				if !ok {
					return
				}
				val.healthy.Store(p.checker.Check(rec))
			}
		}
	}()
}

func (p *HealthCheckFilter) get(key string) *entry {
	val, ok := p.cache.Get(key)
	if !ok {
		return nil
	}

	var result *entry
	result, ok = val.(*entry)
	if !ok {
		return nil
	}

	return result
}
