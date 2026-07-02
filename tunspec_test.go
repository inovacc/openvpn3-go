package openvpn3

import "testing"

func TestTunSpec_BuildsHelperRequests(t *testing.T) {
	var s TunSpec
	s.SetRemoteAddress("203.0.113.5", false)
	s.AddAddress("10.8.0.2", 24, "10.8.0.1", false)
	s.AddRoute("0.0.0.0", 0, 0, false)
	s.AddDNSServer("10.8.0.1")
	s.SetMTU(1500)

	reqs := s.HelperRequests()

	if len(reqs) == 0 {
		t.Fatal("expected helper requests")
	}

	var openIdx, routeIdx = -1, -1

	for i, r := range reqs {
		switch r.Op {
		case "tun-open":
			openIdx = i
		case "add-route":
			routeIdx = i
		}
	}

	if openIdx == -1 || routeIdx == -1 || openIdx > routeIdx {
		t.Fatalf("tun-open must come before add-route: %+v", reqs)
	}
}

func TestTunSpec_DefaultRouteFlag(t *testing.T) {
	var s TunSpec

	s.AddRoute("0.0.0.0", 0, 0, false)

	if !s.RedirectsDefaultGateway() {
		t.Fatal("0.0.0.0/0 route should mark default-gateway redirect")
	}
}
