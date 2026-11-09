package asyncapi

import (
	"net"
	"strings"

	"github.com/charlie-haley/asyncapi-go/asyncapi3"
	"github.com/marmotdata/plugin-sdk/mrn"
)

// brokerProtocols lists the AsyncAPI server protocols and default ports of the providers whose own plugins bind a host to a zone. Other providers have no plugin to resolve a host hint, so their assets stay without a zone.
var brokerProtocols = map[string]struct {
	protocols   []string
	defaultPort string
}{
	"Kafka": {protocols: []string{"kafka", "kafka-secure"}, defaultPort: "9092"},
	"NATS":  {protocols: []string{"nats", "tls"}, defaultPort: "4222"},
}

// channelMRN is the MRN of a channel asset this plugin owns. The channel is qualified by its service because two services can declare a channel of the same name.
func (s *Source) channelMRN(doc *asyncapi3.Document, name string) string {
	return s.config.AssetMRN("Channel", "AsyncAPI", serviceTitle(doc), name)
}

// brokerMRN is the MRN of an asset a broker owns and the spec only describes. The spec cannot say which Marmot zone the broker is, so when exactly one server address is known for the channel it is passed as a host hint and the server resolves the zone. The asset pass and the lineage pass both use it so they cannot drift apart.
func (s *Source) brokerMRN(doc *asyncapi3.Document, channel *asyncapi3.Channel, assetType, provider, name string) string {
	m := mrn.Asset(assetType, provider, name)
	if host := serverHost(doc, channel, provider); host != "" {
		m = m.WithHost(host)
	}
	return m.String()
}

func serviceTitle(doc *asyncapi3.Document) string {
	if doc == nil || doc.Info == nil {
		return ""
	}
	return doc.Info.Title
}

// serverHost returns the host:port of the single server the channel is served from, or "" when the provider has no host binding, the spec names no matching server or the servers disagree.
func serverHost(doc *asyncapi3.Document, channel *asyncapi3.Channel, provider string) string {
	info, ok := brokerProtocols[provider]
	if !ok || doc == nil {
		return ""
	}

	candidates := doc.Servers
	if channel != nil && len(channel.Servers) > 0 {
		candidates = make(map[string]*asyncapi3.Server, len(channel.Servers))
		for _, ref := range channel.Servers {
			if ref == nil {
				continue
			}
			name := ref.Ref[strings.LastIndex(ref.Ref, "/")+1:]
			if server, ok := doc.Servers[name]; ok {
				candidates[name] = server
			}
		}
	}

	hosts := make(map[string]struct{})
	for _, server := range candidates {
		if server == nil || !hasProtocol(info.protocols, server.Protocol) {
			continue
		}
		host := normaliseHost(server.Host, info.defaultPort)
		if host == "" {
			continue
		}
		hosts[host] = struct{}{}
	}

	if len(hosts) != 1 {
		return ""
	}
	for host := range hosts {
		return host
	}
	return ""
}

func hasProtocol(protocols []string, protocol string) bool {
	for _, p := range protocols {
		if strings.EqualFold(p, protocol) {
			return true
		}
	}
	return false
}

// normaliseHost lowercases a server host, adds the default port when none is given and rejects hosts that hold variables or a list of brokers.
func normaliseHost(host, defaultPort string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" || strings.ContainsAny(host, "{},/ ") {
		return ""
	}
	if _, _, err := net.SplitHostPort(host); err == nil {
		return host
	}
	return net.JoinHostPort(strings.Trim(host, "[]"), defaultPort)
}
