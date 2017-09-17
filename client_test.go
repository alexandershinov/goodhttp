package goodhttp_test

import (
	"testing"
	. "github.com/alexandershinov/goodhttp"
	"net"
	url2 "net/url"
	"fmt"
)

type lookupForRequestTest struct {
	MainDns     map[string][]net.IP
	FallbackDns map[string][]net.IP
	host        string
}

func TestNewClient(t *testing.T) {
	c := NewClient()
	if c.DialTimeout != DefaultDialTimeout {
		t.Errorf("Default dial timeout error >> %d != default (%d)", c.DialTimeout, DefaultDialTimeout)
	}
	if c.IdleConnTimeout != DefaultIdleConnTimeout {
		t.Errorf("Default idle timeout error >> %d != default (%d)", c.IdleConnTimeout, DefaultIdleConnTimeout)
	}
	if c.TLSHandshakeTimeout != DefaultTLSHandshakeTimeout {
		t.Errorf("Default dial timeout error >> %d != default (%d)", c.TLSHandshakeTimeout, DefaultTLSHandshakeTimeout)
	}
	if c.Timeout != DefaultConnectionTimeout {
		t.Errorf("Default dial timeout error >> %d != default (%d)", c.Timeout, DefaultConnectionTimeout)
	}
}

func (test *lookupForRequestTest) Do(t *testing.T) {
	var testMainResolver, testFallbackResolver GoodResolver
	testMainResolver.Lookup = func(host string) ([]net.IP, error) {
		url, err := url2.Parse(host)
		if err != nil {
			return make([]net.IP, 0), err
		}
		host = url.Host
		if "" == host {
			host = url.Path
		}
		ipList := test.MainDns[host]
		return ipList, nil
	}
	testFallbackResolver.Lookup = func(host string) ([]net.IP, error) {
		url, err := url2.Parse(host)
		if err != nil {
			return make([]net.IP, 0), err
		}
		host = url.Host
		if "" == host {
			host = url.Path
		}
		ipList := test.FallbackDns[host]
		return ipList, nil
	}
	c := NewClient()
	c.MainResolver = &testMainResolver
	c.MainResolver.Servers = []string{"0.0.0.0"}
	c.FallbackResolver = &testFallbackResolver
	c.FallbackResolver.Servers = []string{"0.0.0.0"}
	host := test.host
	url, err := url2.Parse(host)
	if err != nil {
		t.Fatalf("Host cant be parsed >> %s", test.host)
	}
	host = url.Host
	if "" == host {
		host = url.Path
	}
	testAnswer := c.LookupForRequest(host)
	goodAnswer := make([]net.IP, 0)
	if _, ok := test.MainDns[host]; ok && len(test.MainDns[host]) > 0 {
		goodAnswer = test.MainDns[host]
	} else if _, ok := test.FallbackDns[host]; ok {
		goodAnswer = test.FallbackDns[host]
	}
	if fmt.Sprintf("%v", goodAnswer) != fmt.Sprintf("%v", testAnswer) {
		t.Errorf("IP list error >> %v must be %v", testAnswer, goodAnswer)
	}
}

func TestClient_LookupForRequest(t *testing.T) {
	for name, test := range map[string]lookupForRequestTest{
		"test1": {
			MainDns: map[string][]net.IP{
				"example.com": {net.IPv4(1, 1, 1, 1), net.IPv4(2, 2, 2, 2)},
			},
			FallbackDns: map[string][]net.IP{
				"example.com": {net.IPv4(1, 1, 1, 3), net.IPv4(2, 2, 2, 3)},
			},
			host: "example.com",
		},
		"test1https": {
			MainDns: map[string][]net.IP{
				"example.com": {net.IPv4(1, 1, 1, 1), net.IPv4(2, 2, 2, 2)},
			},
			FallbackDns: map[string][]net.IP{
				"example.com": {net.IPv4(1, 1, 1, 3), net.IPv4(2, 2, 2, 3)},
			},
			host: "https://example.com",
		},
	} {
		t.Run(name, test.Do)
	}
}