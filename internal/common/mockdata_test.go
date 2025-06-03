package common

import (
	"os"
	"strings"
	"testing"
)

func TestProcessMockTemplate(t *testing.T) {
	InitMockData(42) // deterministic output for test
	tests := []struct {
		name         string
		template     string
		wantOneOf    []string // for RandomString: output should be one of these
		wantContains []string // for other tests: output should contain all of these
	}{
		{
			name:      "RandomString",
			template:  `{{RandomString (SliceString "foo" "bar" "baz")}}`,
			wantOneOf: []string{"foo", "bar", "baz"},
		},
		{
			name:         "Person fields",
			template:     `{{FirstName}} {{LastName}} {{Contact.Email}} {{Contact.Phone}}`,
			wantContains: []string{"@"}, // Email should be present
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ProcessMockTemplate(tt.template, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(tt.wantOneOf) > 0 {
				found := false
				for _, want := range tt.wantOneOf {
					if got == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("output %q is not one of expected values %v", got, tt.wantOneOf)
				}
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("output %q does not contain expected substring %q", got, want)
				}
			}
			if tt.name == "Person fields" {
				parts := strings.Fields(got)
				if len(parts) < 4 {
					t.Errorf("expected at least 4 fields in output, got %q", got)
				}
				phone := parts[len(parts)-1]
				if len(phone) == 0 {
					t.Errorf("expected non-empty phone number, got %q", phone)
				}
			}
		})
	}
}

func TestInitMockDataAndReshuffle(_ *testing.T) {
	InitMockData(0)
	ReshuffleMockData()
	// No panic means success; cannot easily assert randomness
}

func TestProcessMockTemplate_Error(t *testing.T) {
	InitMockData(42)
	_, err := ProcessMockTemplate("{{InvalidFunc}}", nil)
	if err == nil {
		t.Error("expected error for invalid template function")
	}
}

func writeTempFile(t *testing.T, content string) string {
	// Use the root-level testdata directory for temp files
	dir := "../../testdata"
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("failed to create testdata dir: %v", err)
	}
	f, err := os.CreateTemp(dir, "*.pem")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	f.Close()
	return f.Name()
}
