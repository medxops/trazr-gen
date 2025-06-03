// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestSeverityTextFromNumber(t *testing.T) {
	tests := []struct {
		name  string
		input int32
		want  string
	}{
		{"Trace", 1, "Trace"},
		{"Trace2", 2, "Trace2"},
		{"Trace3", 3, "Trace3"},
		{"Trace4", 4, "Trace4"},
		{"Debug", 5, "Debug"},
		{"Debug2", 6, "Debug2"},
		{"Debug3", 7, "Debug3"},
		{"Debug4", 8, "Debug4"},
		{"Info", 9, "Info"},
		{"Info2", 10, "Info2"},
		{"Info3", 11, "Info3"},
		{"Info4", 12, "Info4"},
		{"Warn", 13, "Warn"},
		{"Warn2", 14, "Warn2"},
		{"Warn3", 15, "Warn3"},
		{"Warn4", 16, "Warn4"},
		{"Error", 17, "Error"},
		{"Error2", 18, "Error2"},
		{"Error3", 19, "Error3"},
		{"Error4", 20, "Error4"},
		{"Fatal", 21, "Fatal"},
		{"Fatal2", 22, "Fatal2"},
		{"Fatal3", 23, "Fatal3"},
		{"Fatal4", 24, "Fatal4"},
		{"Below range", 0, ""},
		{"Above range", 25, ""},
		{"Negative", -1, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := severityTextFromNumber(tt.input)
			if got != tt.want {
				t.Errorf("severityTextFromNumber(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

type fakeCmd struct {
	template string
}

func (f *fakeCmd) SetHelpTemplate(s string) { f.template = s }

func TestSetHelpTemplateForCmd(t *testing.T) {
	cmd := &fakeCmd{}
	SetHelpTemplateForCmd(cmd)
	if cmd.template == "" {
		t.Error("SetHelpTemplateForCmd did not set the help template")
	}
}
