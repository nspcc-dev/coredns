package healthchecker

import (
	"testing"

	"github.com/coredns/caddy"
)

func TestSetup(t *testing.T) {
	for _, tc := range []struct {
		args  string
		valid bool
	}{
		// not enough args
		{args: "", valid: false},
		{args: "http", valid: false},
		{args: "http:80", valid: false},
		{args: "http:80:3000", valid: false},
		{args: "http 1000", valid: false},
		{args: "http 1000 1000", valid: false},
		// http method params check, 100, 3000, fs.neo.org. are valid cache size, check interval and origin
		{args: "http 100 1s fs.neo.org.", valid: true},
		{args: "http:asdf 100 1s fs.neo.org.", valid: false},
		{args: "http:0 100 1s fs.neo.org.", valid: false},
		{args: "http:80 100 1s fs.neo.org.", valid: true},
		{args: "http:80a 100 1s fs.neo.org.", valid: false},
		{args: "http:80:3000 100 1s fs.neo.org.", valid: true},
		{args: "http:0:3000 100 1s fs.neo.org.", valid: false},
		{args: "http:0:3000a 100 1s fs.neo.org.", valid: false},
		{args: "http:-1:3000 100 1s fs.neo.org.", valid: false},
		{args: "http:80:0 100 1s fs.neo.org.", valid: false},
		// cache size
		{args: "http -1 1s fs.neo.org.", valid: false},
		{args: "http 100a 1s fs.neo.org.", valid: false},
		{args: "http 0 1s fs.neo.org.", valid: false},
		// check interval, test with a valid value is above
		{args: "http 100 0h fs.neo.org.", valid: false},
		{args: "http 100 100 fs.neo.org.", valid: false},
		{args: "http 100 3000a fs.neo.org.", valid: false},
		{args: "http 100 -1m fs.neo.org.", valid: false},
		// origin, test with a valid value is above
		{args: "http 100 3000 fs.neo.org", valid: false},
		// names
		{args: "http 100 3m fs.neo.org. @ kimchi", valid: true},
	} {
		c := caddy.NewTestController("dns", "healthchecker "+tc.args)
		err := setup(c)
		if tc.valid && err != nil {
			t.Fatalf("Expected no errors, but got: %v, test case: %s", err, tc.args)
		} else if !tc.valid && err == nil {
			t.Fatalf("Expected errors, but got: %v, test case: %s", err, tc.args)
		}
	}
}
