package netguard

import (
	"net"
	"testing"
)

func TestPublicIPClassification(t *testing.T) {
	tests := map[string]bool{
		"127.0.0.1":            false,
		"10.0.0.1":             false,
		"172.16.2.3":           false,
		"192.168.1.1":          false,
		"169.254.169.254":      false,
		"100.64.0.1":           false,
		"::1":                  false,
		"fc00::1":              false,
		"1.1.1.1":              true,
		"2606:4700:4700::1111": true,
	}
	for raw, want := range tests {
		if got := isPublicIP(net.ParseIP(raw)); got != want {
			t.Errorf("isPublicIP(%s)=%v want %v", raw, got, want)
		}
	}
}
