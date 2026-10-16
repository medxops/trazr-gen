package e2etest

import (
	"net"
	"testing"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
)

func setReceiverEndpoint(cfg component.Config, endpoint string) {
	receiver := cfg.(*otlpreceiver.Config)
	receiver.Protocols.GRPC.GetOrInsertDefault().NetAddr.Endpoint = endpoint
	receiver.Protocols.HTTP = configoptional.None[otlpreceiver.HTTPConfig]()
}

func getAvailableLocalAddress(t *testing.T) string {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to get available local address: %v", err)
	}
	defer l.Close()
	return l.Addr().String()
}
