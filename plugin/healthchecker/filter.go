package healthchecker

import (
	"time"

	"github.com/hashicorp/golang-lru"
	"github.com/miekg/dns"
	"go.uber.org/atomic"
)

type (
	HealthCheckFilter struct {
		cache    *lru.Cache
		checker  Checker
		interval time.Duration
		names    map[string]struct{}
	}

	entry struct {
		dnsRecord dns.RR
		healthy   *atomic.Bool
		quit      chan struct{}
	}
)

func NewHealthCheckFilter(checker Checker, size int, interval time.Duration, names map[string]struct{}) (*HealthCheckFilter, error) {
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
		names:    names,
	}, nil
}

func (p *HealthCheckFilter) FilterRecords(records []dns.RR) []dns.RR {
	result := make([]dns.RR, 0, len(records))

	for _, r := range records {
		if _, ok := p.names[r.Header().Name]; ok || len(p.names) == 0 {
			e := p.get(r.String())
			if e != nil {
				if e.healthy.Load() {
					result = append(result, e.dnsRecord)
				}
				continue
			}
			p.put(r)
		}
		result = append(result, r)
	}

	return result
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
