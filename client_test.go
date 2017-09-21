package goodhttp_test

import (
	"testing"
	"github.com/stretchr/testify/assert"
	. "github.com/alexandershinov/goodhttp"
	"github.com/kabukky/httpscerts"
	"net"
	url2 "net/url"
	"fmt"
	"io"
	"bytes"
	"net/http"
)

type lookupForRequestTest struct {
	MainDns     map[string][]net.IP
	FallbackDns map[string][]net.IP
	host        string
}

type goodPostTest struct {
	MainDns     map[string][]net.IP
	FallbackDns map[string][]net.IP
	Url         url2.URL
	ContentType string
	Body        io.Reader
	Result      error
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

func newLocalDns(DnsMap map[string][]net.IP) (localResolver *GoodResolver) {
	localResolver = new(GoodResolver)
	localResolver.Lookup = func(host string) ([]net.IP, error) {
		url, err := url2.Parse(host)
		if err != nil {
			return make([]net.IP, 0), err
		}
		host = url.Host
		if "" == host {
			host = url.Path
		}
		ipList := DnsMap[host]
		return ipList, nil
	}
	localResolver.Servers = []string{"0.0.0.0"}
	return
}

func (test *lookupForRequestTest) Do(t *testing.T) {
	c := NewClient()
	c.MainResolver = newLocalDns(test.MainDns)
	c.FallbackResolver = newLocalDns(test.FallbackDns)
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
	assert.Equalf(t, goodAnswer, testAnswer, "IP list error >> %v must be %v\n", testAnswer, goodAnswer)
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
		"test2": {
			MainDns: map[string][]net.IP{
				"example.com": {},
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

func testHandler(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprintf(w, "OK")
}

func (test *goodPostTest) Do(t *testing.T) {
	// Создаём для тестового https сайта example.com сертификат, если его ещё нет
	// (надо добавить в систему сертификат в доверенные, чтобы не ругался на недоверенный источник)
	if err := httpscerts.Check("cert.pem", "key.pem"); err != nil {
		if err = httpscerts.Generate("cert.pem", "key.pem", "example.com"); err != nil {
			t.Fatal(err)
		}
	}
	// Запускаем в горутине тестовый сервер с созданным сертификатом
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", testHandler)
		err := http.ListenAndServeTLS(":3443", "cert.pem", "key.pem", mux)
		if err != nil {
			t.Fatal(err)
		}
	}()
	// Создаём новый клиент, берём из фикстуры главный и запасной ресолверы
	c := NewClient()
	c.MainResolver = newLocalDns(test.MainDns)
	c.FallbackResolver = newLocalDns(test.FallbackDns)
	// Отправляем POST запрос на тестовый сайт
	_, err := c.GoodPost(test.Url.String(), test.ContentType, test.Body)
	assert.Equal(t, test.Result, err, "Vars err and test.Result (error) should be the same.")
}

func TestClient_GoodPost(t *testing.T) {
	exampleUrl, _ := url2.Parse("https://example.com/")
	exampleUrl2, _ := url2.Parse("https://example2.com/")
	testMainDns := map[string][]net.IP{
		"example.com": {net.IPv4(127, 0, 0, 1)},
	}
	for name, test := range map[string]goodPostTest{
		"testPost1": {
			testMainDns,
			testMainDns,
			*exampleUrl,
			"application/json",
			bytes.NewBuffer([]byte(`{"example": "OK"}`)),
			nil,
		},
		"testPost2": {
			testMainDns,
			testMainDns,
			*exampleUrl2,
			"application/json",
			bytes.NewBuffer([]byte(`{"example": "OK"}`)),
			&Error{"Can`t lookup hostname."},
		},
	} {
		t.Run(name, test.Do)
	}
}
