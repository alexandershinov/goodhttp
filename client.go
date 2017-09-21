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
	"github.com/felixge/tcpkeepalive"
)

var (
	mainResolver, fallbackResolver *GoodResolver
)

const (
	DefaultConnectionTimeout   time.Duration = time.Second * 60
	DefaultTLSHandshakeTimeout time.Duration = 1 * time.Second
	DefaultIdleConnTimeout     time.Duration = 1 * time.Second
	DefaultIntervalTimeout     time.Duration = 1 * time.Second
	DefaultFailAfter           int           = 3
	DefaultDialTimeout         time.Duration = 1 * time.Second
	DefaultResolveTimeout      time.Duration = 1 * time.Second
)

type Error struct {
	Text string
}

func (e *Error) Error() string {
	return e.Text
}

func init() {
	rand.Seed(time.Now().Unix())
	mainResolver = NewDefaultResolver(DefaultResolveTimeout)
	fallbackResolver = NewResolver([]string{"8.8.8.8"}, DefaultResolveTimeout)
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
		KeepAlive: DefaultConnectionTimeout,
		Timeout:   c.DialTimeout,
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
	// Если в url строке не указан протокол, используем https
	if ! strings.Contains(url, "://") {
		url = fmt.Sprintf("https://%s", url)
	}
	parsedUrl, err := url2.Parse(url)
	if err != nil {
		panic(err)
	}
	for _, resolver := range []*GoodResolver{c.MainResolver, c.FallbackResolver} {
		if len(ipList) > 0 {
			break
		}
		ipList, err = resolver.Lookup(parsedUrl.Host)
	}
	if err != nil {
		panic(err)
	}
	return
}

// Установка DialContext для транспорта, который будет ресолвить указанный в параметрах метода url в указанный ip
func (c *Client) SetOneIpDealContext(ipAddr string, url string) {
	dialer := &net.Dialer{
		Timeout: c.DialTimeout,
	}
	c.Transport.(*http.Transport).DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		url, err := url2.Parse(addr)
		if err != nil {
			return nil, err
		}
		port := url.Port()
		addr = fmt.Sprintf("%s:%s", ipAddr, port)
		conn, err := dialer.DialContext(ctx, network, addr)
		if err != nil {
			return conn, err
		}
		// Включение для соединения tcp keepalive (idle=1s, interval=1s, fail_after=3)
		tcpkeepalive.EnableKeepAlive(conn)
		tcpkeepalive.SetKeepAlive(conn, DefaultIdleConnTimeout, DefaultFailAfter, DefaultIntervalTimeout)
		return conn, err
	}
}

// Запрос типа POST к случайному ip из известных
// При ошибке одна дополнительная попытка по другому ip
func (c *Client) GoodPost(url string, contentType string, body io.Reader) (resp *http.Response, err error) {
	ipList := c.LookupForRequest(url)
	if len(ipList) == 0 {
		return nil, &Error{"Can`t lookup hostname."}
	}
	i := rand.Intn(len(ipList))
	c.SetOneIpDealContext(ipList[i].String(), url)
	resp, err = c.Post(url, contentType, body)
	if err != nil && len(ipList) > 1 {
		j := rand.Intn(len(ipList) - 1)
		if j >= i {
			j++
		}
		c.SetOneIpDealContext(ipList[j].String(), url)
		resp, err = c.Post(url, contentType, body)
	}
	return
}
