package healthchecker

import (
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

type tmpcheck struct {
}

func (t *tmpcheck) Check(record dns.RR) bool {
	return true
}

func TestPanic(t *testing.T) {
	checker := &tmpcheck{}

	f, err := NewHealthCheckFilter(checker, 2, 200, []Filter{SimpleMatchFilter("abc")})

	require.NoError(t, err)

	a := &dns.A{A: net.ParseIP("127.0.0.1")}
	a2 := &dns.A{A: net.ParseIP("127.0.0.2")}
	a3 := &dns.A{A: net.ParseIP("127.0.0.3")}
	a4 := &dns.A{A: net.ParseIP("127.0.0.4")}

	f.put(a)
	f.put(a2)
	time.Sleep(200 * time.Millisecond)
	f.put(a3)
	time.Sleep(200 * time.Millisecond)
	f.put(a4)

	time.Sleep(time.Second)
}
