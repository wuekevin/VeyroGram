package imgw_test

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

// AD-2 negative test: none of the IM listeners (API, Manager, cluster, gateway) may answer on a non-loopback address of this host, and the
// diagnostic/debug surface must be off. Skips (not passes) when the host has no non-loopback IPv4 address to probe with.
func TestIMListenersAreLoopbackOnlyAndDebugIsOff(t *testing.T) {
	im := startIM(t, "")
	var nonLoop []string
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		if ipn, ok := a.(*net.IPNet); ok && ipn.IP.To4() != nil && !ipn.IP.IsLoopback() && !ipn.IP.IsLinkLocalUnicast() {
			nonLoop = append(nonLoop, ipn.IP.String())
		}
	}
	// sanity: loopback must be reachable, otherwise the probe proves nothing
	for name, port := range im.Ports {
		if name == "cluster" {
			continue // raft transport accepts only after the cluster handshake; loopback reachability is asserted for the others
		}
		var err error
		for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
			var c net.Conn
			if c, err = net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 2*time.Second); err == nil {
				c.Close()
				break
			}
		}
		if err != nil {
			t.Fatalf("sanity: %s port %d must be reachable on loopback: %v", name, port, err)
		}
	}
	if len(nonLoop) == 0 {
		t.Skip("no non-loopback IPv4 address on this host: cannot probe external reachability")
	}
	for _, ip := range nonLoop {
		for name, port := range im.Ports {
			if c, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), time.Second); err == nil {
				c.Close()
				t.Errorf("IM %s listener (port %d) is reachable on non-loopback address %s", name, port, ip)
			}
		}
	}
	for _, path := range []string{"/debug/pprof/", "/debug/diagnostics/events", "/debug/vars"} {
		resp, err := http.Get(im.APIURL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode == 200 {
			t.Errorf("IM API serves %s with 200: debug surface must be disabled in production config", path)
		}
	}
}
