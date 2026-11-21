package clientip

import (
	"net/http"
	"testing"
)

func req(remote string, xff ...string) *http.Request {
	r, _ := http.NewRequest("GET", "/", nil)
	r.RemoteAddr = remote
	for _, v := range xff {
		r.Header.Add("X-Forwarded-For", v)
	}
	return r
}

func TestFrom(t *testing.T) {
	tr, err := ParseTrusted([]string{"10.0.0.0/8", "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		r    *http.Request
		want string
	}{
		{"untrusted peer: header ignored (spoof)", req("203.0.113.9:1234", "1.2.3.4"), "203.0.113.9"},
		{"trusted peer, single hop", req("10.0.0.5:80", "198.51.100.7"), "198.51.100.7"},
		{"client-prepended entry cannot win", req("10.0.0.5:80", "6.6.6.6, 198.51.100.7"), "198.51.100.7"},
		{"chain through two trusted proxies", req("10.0.0.5:80", "198.51.100.7, 10.0.0.9"), "198.51.100.7"},
		{"multiple header lines", req("127.0.0.1:80", "6.6.6.6", "198.51.100.7"), "198.51.100.7"},
		{"trusted peer, no header", req("10.0.0.5:80"), "10.0.0.5"},
		{"malformed hop stops the walk", req("10.0.0.5:80", "6.6.6.6, garbage"), "10.0.0.5"},
		{"empty trust list ignores header", req("10.0.0.5:80", "1.2.3.4"), "10.0.0.5"},
	}
	for _, c := range cases {
		trusted := tr
		if c.name == "empty trust list ignores header" {
			trusted = nil
		}
		if got := From(c.r, trusted); got != c.want {
			t.Errorf("%s: got %s want %s", c.name, got, c.want)
		}
	}
	if _, err := ParseTrusted([]string{"nope"}); err == nil {
		t.Fatal("bad entry must error")
	}
}
