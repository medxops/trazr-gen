// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"math/rand/v2"
	"strings"

	"github.com/brianvoe/gofakeit/v7"
	"go.uber.org/zap"

	"github.com/medxops/trazr-gen/pkg/metrics"
)

// AttributeGenerator handles generation of realistic span attributes
type AttributeGenerator struct {
	config       *Config
	metricConfig *metrics.Config
	logger       *zap.Logger
}

// NewAttributeGenerator creates a new attribute generator
func NewAttributeGenerator(config *Config, metricConfig *metrics.Config, logger *zap.Logger) *AttributeGenerator {
	return &AttributeGenerator{
		config:       config,
		metricConfig: metricConfig,
		logger:       logger,
	}
}

// GenerateSpanAttributes generates realistic attributes for a span
func (ag *AttributeGenerator) GenerateSpanAttributes(span *Span, count int) map[string]any {
	attributes := make(map[string]any)

	// Add service-specific attributes
	ag.addServiceAttributes(attributes, span)

	// Add HTTP attributes for API spans
	if span.Depth == 0 || span.Depth == 1 {
		ag.addHTTPAttributes(attributes, span)
	}

	// Add database attributes for deeper spans
	if span.Depth >= 2 {
		ag.addDatabaseAttributes(attributes, span)
	}

	// Add custom attributes based on config
	for i := 0; i < count; i++ {
		key := ag.getRandomAttributeKey()
		value := ag.getRandomAttributeValue()
		attributes[key] = value
	}

	// Add mock data if enabled
	if ag.metricConfig.IsMockDataEnabled() {
		ag.addMockDataAttributes(attributes)
	}

	return attributes
}

// addServiceAttributes adds service-specific attributes
func (ag *AttributeGenerator) addServiceAttributes(attributes map[string]any, span *Span) {
	attributes["service.name"] = span.Service
	attributes["service.version"] = ag.getRandomVersion()
	attributes["service.instance.id"] = ag.generateInstanceID()
	attributes["service.namespace"] = ag.getRandomNamespace()
}

// addHTTPAttributes adds HTTP-specific attributes
func (ag *AttributeGenerator) addHTTPAttributes(attributes map[string]any, _ *Span) {
	attributes["http.method"] = ag.getRandomHTTPMethod()
	attributes["http.url"] = ag.getRandomURL()
	attributes["http.status_code"] = ag.getRandomStatusCode()
	attributes["http.user_agent"] = ag.getRandomUserAgent()
	attributes["http.request_id"] = ag.generateRequestID()
	attributes["http.route"] = ag.getRandomRoute()
}

// addDatabaseAttributes adds database-specific attributes
func (ag *AttributeGenerator) addDatabaseAttributes(attributes map[string]any, _ *Span) {
	attributes["db.system"] = ag.getRandomDatabase()
	attributes["db.name"] = ag.getRandomDatabaseName()
	attributes["db.operation"] = ag.getRandomDBOperation()
	attributes["db.statement"] = ag.generateSQLStatement()
	attributes["db.connection_string"] = ag.generateConnectionString()
}

// addMockDataAttributes adds mock data using gofakeit
func (ag *AttributeGenerator) addMockDataAttributes(attributes map[string]any) {
	// Use gofakeit to generate realistic mock data
	attributes["mock.user_id"] = gofakeit.UUID()
	attributes["mock.session_id"] = gofakeit.UUID()
	attributes["mock.correlation_id"] = gofakeit.UUID()
	attributes["mock.timestamp"] = gofakeit.Date()
	attributes["mock.ip_address"] = gofakeit.IPv4Address()
	attributes["mock.user_agent"] = gofakeit.UserAgent()
	attributes["mock.email"] = gofakeit.Email()
	attributes["mock.phone"] = gofakeit.Phone()
	attributes["mock.address"] = gofakeit.Address().Address
	attributes["mock.company"] = gofakeit.Company()
	attributes["mock.job_title"] = gofakeit.JobTitle()
	attributes["mock.credit_card"] = gofakeit.CreditCardNumber(&gofakeit.CreditCardOptions{})
}

// Helper methods for generating realistic data

func (ag *AttributeGenerator) getRandomVersion() string {
	versions := []string{"1.0.0", "1.1.0", "1.2.0", "2.0.0", "2.1.0", "3.0.0"}
	return versions[rand.IntN(len(versions))]
}

func (ag *AttributeGenerator) generateInstanceID() string {
	return gofakeit.UUID()
}

func (ag *AttributeGenerator) getRandomNamespace() string {
	// Get the service.namespace template from the config
	if ag.config != nil && ag.config.ResourceAttributes != nil {
		if namespaceTemplate, exists := ag.config.ResourceAttributes["service.namespace"]; exists {
			if templateStr, ok := namespaceTemplate.(string); ok {
				// Parse the template to extract the values
				// Template format: "{{RandomString (SliceString 'PaymentSystem' 'AuthSystem' 'AnalyticsSystem' 'NotificationSystem')}}"
				values := ag.extractValuesFromTemplate(templateStr)
				if len(values) > 0 {
					return values[rand.IntN(len(values))]
				}
			}
		}
	}

	// Fallback to default values if config is not available
	namespaces := []string{"PaymentSystem", "AuthSystem", "AnalyticsSystem", "NotificationSystem", "InventorySystem", "ShippingSystem"}
	return namespaces[rand.IntN(len(namespaces))]
}

// extractValuesFromTemplate extracts the values from a gofakeit template
// Example: "{{RandomString (SliceString 'PaymentSystem' 'AuthSystem' 'AnalyticsSystem' 'NotificationSystem')}}"
// Returns: ["PaymentSystem", "AuthSystem", "AnalyticsSystem", "NotificationSystem"]
func (ag *AttributeGenerator) extractValuesFromTemplate(template string) []string {
	// Look for the pattern: (SliceString 'value1' 'value2' 'value3')
	start := strings.Index(template, "(SliceString")
	if start == -1 {
		return nil
	}

	// Find the opening parenthesis after SliceString
	start = strings.Index(template[start:], "(") + start
	if start == -1 {
		return nil
	}

	// Find the closing parenthesis
	parenCount := 0
	end := -1
	for i := start; i < len(template); i++ {
		if template[i] == '(' {
			parenCount++
		} else if template[i] == ')' {
			parenCount--
			if parenCount == 0 {
				end = i
				break
			}
		}
	}

	if end == -1 {
		return nil
	}

	// Extract the content between parentheses
	content := template[start+1 : end]

	// Split by spaces and extract quoted values
	var values []string
	parts := strings.Fields(content)
	for _, part := range parts {
		// Remove quotes and clean up
		part = strings.Trim(part, "'\"")
		if part != "" && part != "SliceString" {
			values = append(values, part)
		}
	}

	return values
}

func (ag *AttributeGenerator) getRandomHTTPMethod() string {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	return methods[rand.IntN(len(methods))]
}

func (ag *AttributeGenerator) getRandomURL() string {
	urls := []string{
		"https://api.example.com/users",
		"https://api.example.com/orders",
		"https://api.example.com/products",
		"https://api.example.com/payments",
		"https://api.example.com/auth",
	}
	return urls[rand.IntN(len(urls))]
}

func (ag *AttributeGenerator) getRandomStatusCode() int {
	codes := []int{200, 201, 400, 401, 403, 404, 500}
	return codes[rand.IntN(len(codes))]
}

func (ag *AttributeGenerator) getRandomUserAgent() string {
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36",
		"PostmanRuntime/7.32.3",
		"curl/7.88.1",
	}
	return userAgents[rand.IntN(len(userAgents))]
}

func (ag *AttributeGenerator) generateRequestID() string {
	return gofakeit.UUID()
}

func (ag *AttributeGenerator) getRandomRoute() string {
	routes := []string{
		"/api/v1/users",
		"/api/v1/orders",
		"/api/v1/products",
		"/api/v1/payments",
		"/api/v1/auth/login",
		"/api/v1/auth/logout",
	}
	return routes[rand.IntN(len(routes))]
}

func (ag *AttributeGenerator) getRandomDatabase() string {
	dbs := []string{"postgresql", "mysql", "mongodb", "redis", "elasticsearch", "cassandra"}
	return dbs[rand.IntN(len(dbs))]
}

func (ag *AttributeGenerator) getRandomDatabaseName() string {
	names := []string{"users_db", "orders_db", "products_db", "payments_db", "analytics_db"}
	return names[rand.IntN(len(names))]
}

func (ag *AttributeGenerator) getRandomDBOperation() string {
	ops := []string{"SELECT", "INSERT", "UPDATE", "DELETE", "EXECUTE", "COMMIT", "ROLLBACK"}
	return ops[rand.IntN(len(ops))]
}

func (ag *AttributeGenerator) generateSQLStatement() string {
	statements := []string{
		"SELECT * FROM users WHERE id = ?",
		"INSERT INTO orders (user_id, amount) VALUES (?, ?)",
		"UPDATE products SET stock = stock - ? WHERE id = ?",
		"DELETE FROM sessions WHERE expired_at < NOW()",
		"SELECT COUNT(*) FROM users WHERE created_at > ?",
	}
	return statements[rand.IntN(len(statements))]
}

func (ag *AttributeGenerator) generateConnectionString() string {
	hosts := []string{"localhost", "db.example.com", "database.internal"}
	ports := []string{"5432", "3306", "27017", "6379"}
	databases := []string{"users", "orders", "products", "payments"}

	host := hosts[rand.IntN(len(hosts))]
	port := ports[rand.IntN(len(ports))]
	db := databases[rand.IntN(len(databases))]

	return host + ":" + port + "/" + db
}

func (ag *AttributeGenerator) getRandomAttributeKey() string {
	keys := []string{
		"user.id", "session.id", "request.id", "correlation.id", "tenant.id",
		"environment", "region", "datacenter", "rack", "hostname",
		"process.id", "thread.id", "queue.name", "topic.name",
		"cache.key", "cache.miss", "rate.limit.key", "circuit.breaker.state",
	}
	return keys[rand.IntN(len(keys))]
}

func (ag *AttributeGenerator) getRandomAttributeValue() any {
	values := []any{
		"user-123", "session-456", "req-789", "corr-101", "tenant-202",
		"prod", "us-east-1", "dc-01", "rack-01", "host-001",
		rand.IntN(10000), rand.IntN(100), "main-queue", "events-topic",
		"cache-key-123", true, "rate-limit-key", "closed",
		rand.IntN(1000), rand.Float64() * 100, true, false,
	}
	return values[rand.IntN(len(values))]
}
