package geodns

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/bluele/gcache"
)

type checker struct {
	cache  gcache.Cache
	client *http.Client
	schema string
	port   string
}

func newHealthChecker() *checker {
	return &checker{
		cache: gcache.New(1000).Expiration(30 * time.Second).Build(),
		client: &http.Client{
			Timeout: 2 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		schema: "http://",
		port:   "80",
	}
}

func (c *checker) check(endpoints []string) []bool {
	results := make([]bool, len(endpoints))
	wg := sync.WaitGroup{}

	for i := range endpoints {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			health, err := c.cache.Get(endpoints[i])
			if err == nil {
				health, ok := health.(bool)
				if ok {
					results[i] = health
					return
				}
			}

			results[i] = c.checkOne(endpoints[i])
			_ = c.cache.Set(endpoints[i], results[i])
		}(i)
	}
	wg.Wait()

	return results
}

func (c *checker) checkOne(endpoint string) bool {
	response, err := c.client.Get(c.schema + net.JoinHostPort(endpoint, c.port))
	if err != nil {
		log.Debugf(err.Error())
		return false
	}
	_ = response.Body.Close()

	return response.StatusCode < http.StatusInternalServerError
}
