package goodhttp

import (
	"net/http"
	"net"
	"time"
	"io"
	url2 "net/url"
	"math/rand"
	"strings"
	"fmt"
	"context"
	//"github.com/felixge/tcpkeepalive"
)

var (
	mainResolver, fallbackResolver *GoodResolver
)

const (
	DefaultConnectionTimeout   time.Duration = time.Second * 60
	DefaultTLSHandshakeTimeout time.Duration = 1 * time.Second
	DefaultIdleConnTimeout     time.Duration = 1 * time.Second
	DefaultDialTimeout         time.Duration = 1 * time.Second
	DefaultKeepaliveTimeout    time.Duration = 60 * time.Second
)

type Error struct {
	Text string
}

func (e *Error) Error() string {
	return e.Text
}

func init() {
	rand.Seed(time.Now().Unix())
	mainResolver = NewResolverFromResolvConf("/etc/resolv.conf")
	fallbackResolver = NewResolver([]string{"8.8.8.8"})
}

type Client struct {
	http.Client
	MainResolver        *GoodResolver
	FallbackResolver    *GoodResolver
	TLSHandshakeTimeout time.Duration
	IdleConnTimeout     time.Duration
	DialTimeout         time.Duration
}

// Создание нового http клиента с параметрами по умолчанию
func NewClient() *Client {
	var c Client
	c.IdleConnTimeout = DefaultIdleConnTimeout
	c.DialTimeout = DefaultDialTimeout
	c.Transport = &http.Transport{
		// Todo: Уточнить параметры keepalive
		DialContext: (&net.Dialer{
			Timeout: DefaultDialTimeout,
		}).DialContext,
	}
	c.TLSHandshakeTimeout = DefaultTLSHandshakeTimeout
	c.Timeout = DefaultConnectionTimeout
	c.MainResolver = mainResolver
	c.FallbackResolver = fallbackResolver
	c.UpdateTransport()
	return &c
}

// Обновление Transport для клиента. Обновляется для изменения таймаутов.
// Перед вызовом устанавливаются значения таймаутов в самом Client, а эта функция применяет их к Transport.
func (c *Client) UpdateTransport() {
	c.Transport.(*http.Transport).TLSHandshakeTimeout = c.TLSHandshakeTimeout
	c.Transport.(*http.Transport).IdleConnTimeout = c.IdleConnTimeout
	c.Transport.(*http.Transport).DialContext = (&net.Dialer{
		KeepAlive: DefaultKeepaliveTimeout,
		Timeout: c.DialTimeout,
	}).DialContext
}

// Функции для установки значений параметров таймаутов
func (c *Client) SetConnectionTimeout(t time.Duration) {
	c.Timeout = t
}

func (c *Client) SetTransportDialTimeout(t time.Duration) {
	c.DialTimeout = t
}

func (c *Client) SetTransportIdleTimeout(t time.Duration) {
	c.IdleConnTimeout = t
}

func (c *Client) SetTransportTLSHandshakeTimeout(t time.Duration) {
	c.TLSHandshakeTimeout = t
}

// Определение списка адресов, в которые ресолвится хост переданного параметра url
func (c *Client) LookupForRequest(url string) (ipList []net.IP) {
	var err error
	if ! strings.Contains(url, "://") {
		url = fmt.Sprintf("https://%s", url)
	}
	parsedUrl, err := url2.Parse(url)
	if err != nil {
		panic(err)
	}
	resolver := c.MainResolver
	if resolver != nil && len(resolver.Servers) > 0 {
		ipList, err = resolver.Lookup(parsedUrl.Host)
	}
	if len(ipList) == 0 {
		resolver = c.FallbackResolver
		if resolver == nil || len(resolver.Servers) == 0 {
			panic("Empty fallback resolver.")
		}
		if ipList, err = resolver.Lookup(parsedUrl.Host); err != nil {
			panic(err)
		}
	}
	return
}

// Запрос типа POST к случайному ip из известных
// При ошибке одна дополнительная попытка по другому ip
func (c *Client) GoodPost(url string, contentType string, body io.Reader) (resp *http.Response, err error) {
	ipList := c.LookupForRequest(url)
	dialer := &net.Dialer{
		Timeout: c.DialTimeout,
	}
	if len(ipList) == 0 {
		return nil, &Error{"Can`t lookup hostname."}
	}
	i := rand.Intn(len(ipList))
	c.Transport.(*http.Transport).DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if strings.Contains(url, strings.Replace(addr, ":443", "", 1)) {
			addr = ipList[i].String() + ":443"
		}
		conn, err := dialer.DialContext(ctx, network, addr)
		if err != nil {
			return conn, err
		}
		//tcpkeepalive.EnableKeepAlive(conn)
		//tcpkeepalive.SetKeepAlive(conn, 1*time.Second, 3, 1*time.Second)
		return conn, err
	}
	resp, err = c.Post(url, contentType, body)
	if err != nil && len(ipList) > 1 {
		j := rand.Intn(len(ipList) - 1)
		if j >= i {
			j++
		}
		c.Transport.(*http.Transport).DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			if strings.Contains(url, strings.Replace(addr, ":443", "", 1)) {
				addr = ipList[j].String() + ":443"
			}
			conn, err := dialer.DialContext(ctx, network, addr)
			if err != nil {
				return conn, err
			}
			//tcpkeepalive.EnableKeepAlive(conn)
			//tcpkeepalive.SetKeepAlive(conn, 1*time.Second, 3, 1*time.Second)
			return conn, err
		}
		resp, err = c.Post(url, contentType, body)
	}
	return
}
