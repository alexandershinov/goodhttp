package goodhttp

import (
	"net/http"
	"github.com/bogdanovich/dns_resolver"
	"net"
	"time"
	"io"
	url2 "net/url"
	"math/rand"
	// "fmt"
	"strings"
	"fmt"
	"context"
)

var mainResolver, fallbackResolver *dns_resolver.DnsResolver

const defaultConnectionTimeout time.Duration = time.Second * 5

func init() {
	rand.Seed(time.Now().Unix())
	mainResolver, _ = dns_resolver.NewFromResolvConf("/etc/resolv.conf")
	fallbackResolver = dns_resolver.New([]string{"8.8.8.8"})
}

type Client struct {
	http.Client
	MainResolver        *dns_resolver.DnsResolver
	FallbackResolver    *dns_resolver.DnsResolver
	TLSHandshakeTimeout time.Duration
	IdleConnTimeout     time.Duration
	DialTimeout         time.Duration
}

// Создание нового http клиента с параметрами по умолчанию
func NewClient() *Client {
	var c Client
	c.IdleConnTimeout = 1 * time.Second
	c.DialTimeout = 1 * time.Second
	c.TLSHandshakeTimeout = 1 * time.Second
	c.Timeout = defaultConnectionTimeout
	c.MainResolver = mainResolver
	c.FallbackResolver = fallbackResolver
	c.UpdateTransport()
	return &c
}

// Обновление Transport для клиента. Обновляется для изменения таймаутов.
// Перед вызовом устанавливаются значения таймаутов в самом Client, а эта функция применяет их к Transport.
func (c *Client) UpdateTransport() {
	transport := http.Transport{
		// Todo: Уточнить параметры keepalive
		Dial: (&net.Dialer{
			Timeout: c.DialTimeout,
		}).Dial,
		TLSHandshakeTimeout: c.TLSHandshakeTimeout,
		IdleConnTimeout: c.IdleConnTimeout,
	}
	c.Transport = &transport
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
func (c *Client) LookupForRequest(url string) (urlList []string) {
	var ipList []net.IP
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
		ipList, err = resolver.LookupHost(parsedUrl.Host)
	}
	if len(ipList) == 0 {
		resolver = c.FallbackResolver
		if resolver == nil || len(resolver.Servers) == 0 {
			panic("Empty fallback resolver.")
		}
		if ipList, err = resolver.LookupHost(parsedUrl.Host); err != nil {
			panic(err)
		}
	}
	for i := 0; i < len(ipList); i++ {
		parsedUrl.Host = ipList[i].String()
		urlList = append(urlList, parsedUrl.Host)
	}
	return
}

// Запрос типа POST к случайному ip из известных
// При ошибке одна дополнительная попытка по другому ip
func (c *Client) GoodPost(url string, contentType string, body io.Reader) (resp *http.Response, err error) {
	urlsList := c.LookupForRequest(url)
	// fmt.Println(urlsList)
	dialer := &net.Dialer{
		Timeout:   c.DialTimeout,
	}
	i := rand.Intn(len(urlsList))
	c.Transport.(*http.Transport).DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if strings.Contains(url, strings.Replace(addr, ":443", "", 1)) {
			addr = urlsList[i] + ":443"
		}
		return dialer.DialContext(ctx, network, addr)
	}
	resp, err = c.Post(url, contentType, body)
	if err != nil {
		j := rand.Intn(len(urlsList) - 1)
		if j >= i {
			j++
		}
		c.Transport.(*http.Transport).DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			if strings.Contains(url, strings.Replace(addr, ":443", "", 1)) {
				addr = urlsList[j] + ":443"
			}
			return dialer.DialContext(ctx, network, addr)
		}
		resp, err = c.Post(url, contentType, body)
	}
	return
}
