// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"math/rand/v2"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"go.uber.org/zap"

	"github.com/medxops/trazr-gen/pkg/logs"
)

// EventGenerator handles generation of realistic span events
type EventGenerator struct {
	logConfig *logs.Config
	logger    *zap.Logger
}

// NewEventGenerator creates a new event generator
func NewEventGenerator(logConfig *logs.Config, logger *zap.Logger) *EventGenerator {
	return &EventGenerator{
		logConfig: logConfig,
		logger:    logger,
	}
}

// GenerateSpanEvents generates realistic events for a span
func (eg *EventGenerator) GenerateSpanEvents(span *Span, count int) []SpanEvent {
	events := make([]SpanEvent, 0, count)

	for i := 0; i < count; i++ {
		event := eg.generateSingleEvent(span)
		events = append(events, event)
	}

	return events
}

// generateSingleEvent generates a single realistic event
func (eg *EventGenerator) generateSingleEvent(span *Span) SpanEvent {
	// Generate timestamp within span duration
	spanDuration := span.EndTime.Sub(span.StartTime)
	eventTime := span.StartTime.Add(time.Duration(rand.Int64N(int64(spanDuration))))

	// Generate event name
	eventNames := []string{"cache.hit", "cache.miss", "db.query", "external.call", "validation.failed"}
	eventName := eventNames[rand.IntN(len(eventNames))]

	// Generate event attributes
	attributes := make(map[string]any)
	switch eventName {
	case "cache.hit":
		keys := []string{"user.profile", "session.data", "config.settings", "rate.limit", "auth.token"}
		attributes["cache.key"] = keys[rand.IntN(len(keys))]
		attributes["cache.ttl"] = rand.IntN(3600) + 60
		attributes["cache.size"] = rand.IntN(1024) + 1
	case "cache.miss":
		keys := []string{"user.profile", "session.data", "config.settings", "rate.limit", "auth.token"}
		attributes["cache.key"] = keys[rand.IntN(len(keys))]
		attributes["cache.reason"] = "expired"
	case "db.query":
		statements := []string{
			"SELECT * FROM users WHERE id = ?",
			"INSERT INTO orders (user_id, amount) VALUES (?, ?)",
			"UPDATE products SET stock = stock - ? WHERE id = ?",
			"DELETE FROM sessions WHERE expired_at < ?",
			"EXECUTE stored_procedure(?, ?)",
		}
		attributes["db.statement"] = statements[rand.IntN(len(statements))]
		attributes["db.rows_affected"] = rand.IntN(1000)
		attributes["db.duration_ms"] = rand.IntN(100) + 1
	case "external.call":
		services := []string{"payment.gateway", "email.service", "sms.provider", "analytics.api", "notification.service"}
		attributes["external.service"] = services[rand.IntN(len(services))]
		attributes["external.endpoint"] = "/api/v1/process"
		attributes["external.duration_ms"] = rand.IntN(500) + 10
	case "validation.failed":
		fields := []string{"email", "password", "amount", "user_id", "product_id"}
		attributes["validation.field"] = fields[rand.IntN(len(fields))]
		attributes["validation.error"] = "invalid format"
	}

	event := SpanEvent{
		Name:       eventName,
		Timestamp:  eventTime,
		Attributes: attributes,
	}

	// Add event-specific attributes
	eg.addEventAttributes(&event, span)

	return event
}

// addEventAttributes adds realistic attributes based on event type
func (eg *EventGenerator) addEventAttributes(event *SpanEvent, span *Span) {
	// Common attributes
	event.Attributes["event.type"] = "custom"
	event.Attributes["event.source"] = span.Service

	// Event-specific attributes
	switch event.Name {
	case "cache.hit", "cache.miss":
		event.Attributes["cache.key"] = eg.generateCacheKey()
		event.Attributes["cache.ttl"] = rand.IntN(3600) + 60

	case "db.query", "db.connection":
		event.Attributes["db.system"] = eg.getRandomDatabase()
		event.Attributes["db.operation"] = eg.getRandomDBOperation()
		event.Attributes["db.statement"] = eg.generateSQLStatement()

	case "external.call":
		event.Attributes["http.method"] = eg.getRandomHTTPMethod()
		event.Attributes["http.url"] = eg.getRandomExternalURL()
		event.Attributes["http.status_code"] = eg.getRandomStatusCode()

	case "validation.failed":
		event.Attributes["validation.field"] = eg.getRandomFieldName()
		event.Attributes["validation.error"] = eg.getRandomValidationError()

	case "authorization.check":
		event.Attributes["auth.user"] = eg.generateUserID()
		event.Attributes["auth.role"] = eg.getRandomRole()
		event.Attributes["auth.permission"] = eg.getRandomPermission()

	case "rate.limit":
		event.Attributes["rate.limit.key"] = eg.generateRateLimitKey()
		event.Attributes["rate.limit.limit"] = rand.IntN(1000)
		event.Attributes["rate.limit.remaining"] = rand.IntN(100)

	case "circuit.breaker":
		event.Attributes["circuit.breaker.state"] = eg.getRandomCircuitState()
		event.Attributes["circuit.breaker.failure_count"] = rand.IntN(10)

	case "retry.attempt":
		event.Attributes["retry.attempt"] = rand.IntN(5)
		event.Attributes["retry.max_attempts"] = 3
	}

	// Add mock data if enabled
	if eg.logConfig.IsMockDataEnabled() {
		eg.addMockDataAttributes(event)
	}
}

// addMockDataAttributes adds mock data using gofakeit
func (eg *EventGenerator) addMockDataAttributes(event *SpanEvent) {
	// Use gofakeit to generate realistic mock data
	event.Attributes["mock.user_id"] = gofakeit.UUID()
	event.Attributes["mock.session_id"] = gofakeit.UUID()
	event.Attributes["mock.request_id"] = gofakeit.UUID()
	event.Attributes["mock.correlation_id"] = gofakeit.UUID()
	event.Attributes["mock.timestamp"] = gofakeit.Date()
	event.Attributes["mock.ip_address"] = gofakeit.IPv4Address()
	event.Attributes["mock.user_agent"] = gofakeit.UserAgent()
}

// Helper methods for generating realistic data

func (eg *EventGenerator) generateCacheKey() string {
	keys := []string{
		"user:profile:123", "product:details:456", "session:data:789",
		"config:app:settings", "rate:limit:ip:192.168.1.1",
	}
	return keys[rand.IntN(len(keys))]
}

func (eg *EventGenerator) getRandomDatabase() string {
	dbs := []string{"postgresql", "mysql", "mongodb", "redis", "elasticsearch", "cassandra"}
	return dbs[rand.IntN(len(dbs))]
}

func (eg *EventGenerator) getRandomDBOperation() string {
	ops := []string{"SELECT", "INSERT", "UPDATE", "DELETE", "EXECUTE", "COMMIT", "ROLLBACK"}
	return ops[rand.IntN(len(ops))]
}

func (eg *EventGenerator) generateSQLStatement() string {
	statements := []string{
		"SELECT * FROM users WHERE id = ?",
		"INSERT INTO orders (user_id, amount) VALUES (?, ?)",
		"UPDATE products SET stock = stock - ? WHERE id = ?",
		"DELETE FROM sessions WHERE expired_at < NOW()",
	}
	return statements[rand.IntN(len(statements))]
}

func (eg *EventGenerator) getRandomHTTPMethod() string {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	return methods[rand.IntN(len(methods))]
}

func (eg *EventGenerator) getRandomExternalURL() string {
	urls := []string{
		"https://api.payment-gateway.com/charge",
		"https://api.email-service.com/send",
		"https://api.notification-service.com/push",
		"https://api.analytics-service.com/track",
	}
	return urls[rand.IntN(len(urls))]
}

func (eg *EventGenerator) getRandomStatusCode() int {
	codes := []int{200, 201, 400, 401, 403, 404, 429, 500, 502, 503}
	return codes[rand.IntN(len(codes))]
}

func (eg *EventGenerator) getRandomFieldName() string {
	fields := []string{"email", "password", "username", "phone", "address", "age"}
	return fields[rand.IntN(len(fields))]
}

func (eg *EventGenerator) getRandomValidationError() string {
	errors := []string{
		"Field is required", "Invalid format", "Value too long", "Value too short",
		"Invalid email format", "Password too weak", "Age must be positive",
	}
	return errors[rand.IntN(len(errors))]
}

func (eg *EventGenerator) generateUserID() string {
	return gofakeit.UUID()
}

func (eg *EventGenerator) getRandomRole() string {
	roles := []string{"user", "admin", "moderator", "guest", "premium"}
	return roles[rand.IntN(len(roles))]
}

func (eg *EventGenerator) getRandomPermission() string {
	permissions := []string{"read", "write", "delete", "admin", "moderate"}
	return permissions[rand.IntN(len(permissions))]
}

func (eg *EventGenerator) generateRateLimitKey() string {
	keys := []string{
		"ip:192.168.1.1", "user:12345", "api:key:abc123", "session:xyz789",
	}
	return keys[rand.IntN(len(keys))]
}

func (eg *EventGenerator) getRandomCircuitState() string {
	states := []string{"closed", "open", "half-open"}
	return states[rand.IntN(len(states))]
}
