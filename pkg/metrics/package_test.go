// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	// Configure goleak to ignore HTTP-related goroutines that are expected in tests
	opts := []goleak.Option{
		goleak.IgnoreTopFunction("net/http.(*Transport).dialConn"),
		goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"),
		goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
		goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
	}
	goleak.VerifyTestMain(m, opts...)
}
