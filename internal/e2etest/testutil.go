package e2etest

import (
	"net"
	"testing"
)

func getAvailableLocalAddress(t *testing.T) string {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to get available local address: %v", err)
	}
	defer l.Close()
	return l.Addr().String()
}
