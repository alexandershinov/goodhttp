package goodhttp

import (
	"github.com/bogdanovich/dns_resolver"
	"net"
	"time"
	"errors"
)

type GoodResolver struct {
	dns_resolver.DnsResolver
	Lookup func(host string) ([]net.IP, error)
}

type ResolverInfo struct {
	ipList []net.IP
	Error  error
}

func ResolveTimeoutDecorator(f func(host string) ([]net.IP, error), t time.Duration) func(host string) ([]net.IP, error) {
	return func(host string) ([]net.IP, error) {
		c := make(chan ResolverInfo)
		go func() {
			var resolverInfo ResolverInfo
			resolverInfo.ipList, resolverInfo.Error = f(host)
			c <- resolverInfo
		}()
		select {
		case res := <-c:
			return res.ipList, res.Error
		case <-time.After(t):
			return []net.IP{}, errors.New("Resolve timeout error.")
		}
	}
}

func NewDefaultResolver(timeout time.Duration) *GoodResolver {
	r := ResolveTimeoutDecorator(net.LookupIP, timeout)
	return &GoodResolver{dns_resolver.DnsResolver{}, r}
}

func NewResolverFromResolvConf(resolvConfFile string, timeout time.Duration) *GoodResolver {
	dnsResolver, _ := dns_resolver.NewFromResolvConf(resolvConfFile)
	r := ResolveTimeoutDecorator(dnsResolver.LookupHost, timeout)
	var resolver GoodResolver = GoodResolver{*dnsResolver, r}
	return &resolver
}

func NewResolver(servers []string, timeout time.Duration) *GoodResolver {
	dnsResolver := dns_resolver.New(servers)
	r := ResolveTimeoutDecorator(dnsResolver.LookupHost, timeout)
	var resolver GoodResolver = GoodResolver{*dnsResolver, r}
	return &resolver
}
