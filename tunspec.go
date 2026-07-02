package openvpn3

import (
	"strconv"
	"strings"
)

// HelperRequest is one privileged op to send to lensr's elevated vpnhelper.
// Op values map to existing vpnhelper verbs (tun-open, add-route, set-dns,
// flushdns, tun-close). Args are op-specific key/values.
type HelperRequest struct {
	Op   string
	Args map[string]string
}

// TunSpec accumulates TunBuilderBase verbs from OpenVPN3 and renders them into
// an ordered list of HelperRequests. Pure data; no I/O. The cgo tun bridge
// feeds verbs in and ships HelperRequests() to the pipe on establish().
//
// INERT on Windows — OpenVPN3 3.11.6 does not invoke TunBuilderBase on Windows
// (USE_TUN_BUILDER is Android/iOS-only), so these TunBuilder-translation helpers
// are not exercised by the Windows OpenVPN3 path. Retained for non-Windows/future
// use and unit-tested in isolation.
type TunSpec struct {
	remote    string
	remoteV6  bool
	addrs     []tunAddr
	routes    []tunRoute
	dns       []string
	mtu       int
	defaultGW bool
}

type tunAddr struct {
	addr    string
	prefix  int
	gateway string
	ipv6    bool
}

type tunRoute struct {
	addr   string
	prefix int
	metric int
	ipv6   bool
}

func (s *TunSpec) SetRemoteAddress(addr string, ipv6 bool) { s.remote, s.remoteV6 = addr, ipv6 }

func (s *TunSpec) AddAddress(addr string, prefix int, gateway string, ipv6 bool) {
	s.addrs = append(s.addrs, tunAddr{addr, prefix, gateway, ipv6})
}

func (s *TunSpec) AddRoute(addr string, prefix, metric int, ipv6 bool) {
	if addr == "0.0.0.0" && prefix == 0 {
		s.defaultGW = true
	}

	s.routes = append(s.routes, tunRoute{addr, prefix, metric, ipv6})
}

func (s *TunSpec) AddDNSServer(ip string) { s.dns = append(s.dns, ip) }
func (s *TunSpec) SetMTU(mtu int)         { s.mtu = mtu }

// RedirectsDefaultGateway reports whether a 0.0.0.0/0 route was added.
func (s *TunSpec) RedirectsDefaultGateway() bool { return s.defaultGW }

// HelperRequests renders the accumulated spec into ordered privileged ops.
// Order: tun-open (with addrs+mtu) -> add-route* -> set-dns.
func (s *TunSpec) HelperRequests() []HelperRequest {
	reqs := make([]HelperRequest, 0, len(s.routes)+2)

	open := HelperRequest{Op: "tun-open", Args: map[string]string{}}

	if len(s.addrs) > 0 {
		a := s.addrs[0]
		open.Args["address"] = a.addr
		open.Args["prefix"] = strconv.Itoa(a.prefix)
		open.Args["gateway"] = a.gateway
	}

	if s.mtu > 0 {
		open.Args["mtu"] = strconv.Itoa(s.mtu)
	}

	reqs = append(reqs, open)

	for _, r := range s.routes {
		reqs = append(reqs, HelperRequest{Op: "add-route", Args: map[string]string{
			"address": r.addr,
			"prefix":  strconv.Itoa(r.prefix),
			"metric":  strconv.Itoa(r.metric),
		}})
	}

	if len(s.dns) > 0 {
		reqs = append(reqs, HelperRequest{Op: "set-dns", Args: map[string]string{
			"servers": strings.Join(s.dns, ","),
		}})
	}

	return reqs
}
