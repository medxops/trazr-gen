package cli

import (
	"strings"
	"testing"

	"github.com/urfave/cli/v2"
)

func TestGenerateLogs_MissingEndpoint(t *testing.T) {
	app := cli.NewApp()
	app.Flags = getGlobalFlags()
	app.Commands = []*cli.Command{genLogsCommand()}
	// Simulate: trazr-gen logs single (no endpoint)
	err := app.Run([]string{"trazr-gen", "logs", "single"})
	if err == nil || !strings.Contains(err.Error(), "endpoint") {
		t.Errorf("expected error for missing endpoint, got: %v", err)
	}
}
