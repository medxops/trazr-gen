package common

import (
	"testing"
)

func TestCaPool(t *testing.T) {
	pem := `-----BEGIN CERTIFICATE-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAnzQw6Q==\n-----END CERTIFICATE-----`
	caFile := writeTempFile(t, pem)
	_, err := caPool(caFile)
	if err == nil {
		t.Error("caPool should error on invalid PEM")
	}
	_, err = caPool("/nonexistent/file.pem")
	if err == nil {
		t.Error("caPool should error on missing file")
	}
}

func TestGetTLSCredentialsForGRPCExporter(t *testing.T) {
	_, err := GetTLSCredentialsForGRPCExporter("", ClientAuth{}, false)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetTLSCredentialsForHTTPExporter(t *testing.T) {
	cfg, err := GetTLSCredentialsForHTTPExporter("", ClientAuth{}, false)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Error("expected non-nil tls.Config")
	}
}

func TestGetTLSConfig_mTLS(t *testing.T) {
	certPEM := `-----BEGIN CERTIFICATE-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAnzQw6Q==\n-----END CERTIFICATE-----`
	keyPEM := "TEST-PRIVATE-KEY-PLACEHOLDER"
	certFile := writeTempFile(t, certPEM)
	keyFile := writeTempFile(t, keyPEM)
	cAuth := ClientAuth{Enabled: true, ClientCertFile: certFile, ClientKeyFile: keyFile}
	_, err := GetTLSCredentialsForHTTPExporter("", cAuth, false)
	if err == nil {
		t.Error("expected error for invalid cert/key, got nil")
	}
}
