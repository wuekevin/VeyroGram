package domain

import "testing"

func TestParsePhone(t *testing.T) {
	for in, want := range map[string]string{"+86 138-0013-8000": "+8613800138000", "+14155550123": "+14155550123"} {
		if p, err := ParsePhone(in); err != nil || string(p) != want {
			t.Errorf("%q: %v %v", in, p, err)
		}
	}
	for _, in := range []string{"", "13800138000", "+0123456789", "+1234567", "+1234567890123456", "+86a3800138000", "+86 138 0013 8000; DROP"} {
		if _, err := ParsePhone(in); err == nil {
			t.Errorf("%q must be rejected", in)
		}
	}
	if p, _ := ParsePhone("+8613800138000"); p.Last4() != "8000" {
		t.Error("last4")
	}
}

func TestParseDevice(t *testing.T) {
	if d, err := ParseDevice("device-aaaaaaaaaaaaaaaa", "web", "My\x00 Mac\n"); err != nil || d.Name != "My Mac" || d.Class != ClassWeb {
		t.Fatalf("%+v %v", d, err)
	}
	for _, c := range [][3]string{{"short", "app", ""}, {"device-aaaaaaaaaaaaaaaa", "tv", ""}, {"device aaaaaaaaaaaaaaa!", "app", ""}} {
		if _, err := ParseDevice(c[0], c[1], c[2]); err == nil {
			t.Errorf("%v must be rejected", c)
		}
	}
	long := make([]rune, 300)
	for i := range long {
		long[i] = '设'
	}
	if d, _ := ParseDevice("device-aaaaaaaaaaaaaaaa", "app", string(long)); len([]rune(d.Name)) != 100 {
		t.Error("name must be truncated to 100 runes")
	}
}
