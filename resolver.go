package goodhttp

import (
	"github.com/bogdanovich/dns_resolver"
	"net"
)

type GoodResolver struct {
	dns_resolver.DnsResolver
	Lookup func(host string) ([]net.IP, error)
}

func NewResolverFromResolvConf(resolvConfFile string) *GoodResolver {
	dnsResolver, _ := dns_resolver.NewFromResolvConf(resolvConfFile)
	var resolver GoodResolver = GoodResolver{*dnsResolver, dnsResolver.LookupHost}
	return &resolver
}

func NewResolver(servers []string) *GoodResolver {
	dnsResolver := dns_resolver.New(servers)
	var resolver GoodResolver = GoodResolver{*dnsResolver, dnsResolver.LookupHost}
	return &resolver
}
