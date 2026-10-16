// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"testing"

	"go.uber.org/zap"

	"github.com/medxops/trazr-gen/pkg/metrics"
)

func TestAttributeGenerator_GenerateSpanAttributes(t *testing.T) {
	metricConfig := &metrics.Config{}
	logger := zap.NewNop()
	config := &Config{}

	generator := NewAttributeGenerator(config, metricConfig, logger)

	span := &Span{
		Service: "test-service",
		Depth:   0,
	}

	attributes := generator.GenerateSpanAttributes(span, 5)

	// Check that service attributes are present
	if _, exists := attributes["service.name"]; !exists {
		t.Error("service.name attribute not found")
	}
	if _, exists := attributes["service.version"]; !exists {
		t.Error("service.version attribute not found")
	}
	if _, exists := attributes["service.instance.id"]; !exists {
		t.Error("service.instance.id attribute not found")
	}
	if _, exists := attributes["service.namespace"]; !exists {
		t.Error("service.namespace attribute not found")
	}

	// Check that we have the expected number of attributes
	expectedMin := 9 // 4 service attributes + 5 random attributes
	if len(attributes) < expectedMin {
		t.Errorf("expected at least %d attributes, got %d", expectedMin, len(attributes))
	}
}

func TestAttributeGenerator_AddHTTPAttributes(t *testing.T) {
	metricConfig := &metrics.Config{}
	logger := zap.NewNop()
	config := &Config{}

	generator := NewAttributeGenerator(config, metricConfig, logger)

	span := &Span{
		Service: "test-service",
		Depth:   0,
	}

	attributes := generator.GenerateSpanAttributes(span, 0)

	// Check that HTTP attributes are present for depth 0
	httpAttributes := []string{"http.method", "http.url", "http.status_code", "http.user_agent", "http.request_id", "http.route"}
	for _, attr := range httpAttributes {
		if _, exists := attributes[attr]; !exists {
			t.Errorf("HTTP attribute %s not found", attr)
		}
	}
}

func TestAttributeGenerator_AddDatabaseAttributes(t *testing.T) {
	metricConfig := &metrics.Config{}
	logger := zap.NewNop()
	config := &Config{}

	generator := NewAttributeGenerator(config, metricConfig, logger)

	span := &Span{
		Service: "test-service",
		Depth:   2, // Deep enough to trigger database attributes
	}

	attributes := generator.GenerateSpanAttributes(span, 0)

	// Check that database attributes are present for depth >= 2
	dbAttributes := []string{"db.system", "db.name", "db.operation", "db.statement", "db.connection_string"}
	for _, attr := range dbAttributes {
		if _, exists := attributes[attr]; !exists {
			t.Errorf("Database attribute %s not found", attr)
		}
	}
}

func TestAttributeGenerator_AddMockDataAttributes(t *testing.T) {
	metricConfig := &metrics.Config{}
	metricConfig.MockData = true // Set mock data enabled
	logger := zap.NewNop()
	config := &Config{}

	generator := NewAttributeGenerator(config, metricConfig, logger)

	span := &Span{
		Service: "test-service",
		Depth:   0,
	}

	attributes := generator.GenerateSpanAttributes(span, 0)

	// Check that mock data attributes are present when mock data is enabled
	mockAttributes := []string{"mock.user_id", "mock.session_id", "mock.correlation_id", "mock.timestamp", "mock.ip_address", "mock.user_agent", "mock.email", "mock.phone", "mock.address", "mock.company", "mock.job_title", "mock.credit_card"}
	for _, attr := range mockAttributes {
		if _, exists := attributes[attr]; !exists {
			t.Errorf("Mock data attribute %s not found", attr)
		}
	}
}

func TestAttributeGenerator_NoMockDataAttributes(t *testing.T) {
	metricConfig := &metrics.Config{}
	metricConfig.MockData = false // Set mock data disabled
	logger := zap.NewNop()
	config := &Config{}

	generator := NewAttributeGenerator(config, metricConfig, logger)

	span := &Span{
		Service: "test-service",
		Depth:   0,
	}

	attributes := generator.GenerateSpanAttributes(span, 0)

	// Check that mock data attributes are NOT present when mock data is disabled
	mockAttributes := []string{"mock.user_id", "mock.session_id", "mock.correlation_id", "mock.timestamp", "mock.ip_address", "mock.user_agent", "mock.email", "mock.phone", "mock.address", "mock.company", "mock.job_title", "mock.credit_card"}
	for _, attr := range mockAttributes {
		if _, exists := attributes[attr]; exists {
			t.Errorf("Mock data attribute %s found when mock data is disabled", attr)
		}
	}
}

func TestAttributeGenerator_ServiceNamespaceValues(t *testing.T) {
	metricConfig := &metrics.Config{}
	logger := zap.NewNop()
	config := &Config{}

	generator := NewAttributeGenerator(config, metricConfig, logger)

	span := &Span{
		Service: "test-service",
		Depth:   0,
	}

	attributes := generator.GenerateSpanAttributes(span, 0)

	// Check that service.namespace is one of the expected values
	namespace, exists := attributes["service.namespace"]
	if !exists {
		t.Error("service.namespace attribute not found")
	}

	expectedNamespaces := []string{"PaymentSystem", "AuthSystem", "AnalyticsSystem", "NotificationSystem", "InventorySystem", "ShippingSystem"}
	namespaceStr, ok := namespace.(string)
	if !ok {
		t.Error("service.namespace is not a string")
	}

	found := false
	for _, expected := range expectedNamespaces {
		if namespaceStr == expected {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("service.namespace value '%s' is not one of the expected values: %v", namespaceStr, expectedNamespaces)
	}
}
