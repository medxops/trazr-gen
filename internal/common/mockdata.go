package common

import (
	"fmt"
	"sync"
	"time"

	"github.com/brianvoe/gofakeit/v7"
)

// fakerInstance holds the active gofakeit.Faker instance that will be used
// as the primary context for template processing.
// This allows all template calls for a "session" to draw from the same random sequence.
var (
	fakerInstance *gofakeit.Faker
	fakerMutex    sync.RWMutex // Protects fakerInstance for concurrent access
)

// InitMockData initializes the gofakeit seed for mock data generation.
// It sets the seed for the *shared* Faker instance used by ProcessMockTemplate.
// Call this once at startup if you need reproducible mock data for a specific test run.
func InitMockData(seed int64) {
	fakerMutex.Lock()
	if seed == 0 {
		// Mask to ensure positive value, safe for gofakeit.New
		fakerInstance = gofakeit.New(uint64(time.Now().UnixNano() & 0x7FFFFFFFFFFFFFFF)) //nolint:gosec // masking ensures safe conversion
	} else {
		fakerInstance = gofakeit.New(uint64(seed & 0x7FFFFFFFFFFFFFFF)) //nolint:gosec // masking ensures safe conversion
	}
	fakerMutex.Unlock()
}

// ReshuffleMockData generates a new seed based on the current time,
// creating a new gofakeit.Faker instance. This effectively provides a "new set"
// of random data for subsequent template calls.
func ReshuffleMockData() {
	fakerMutex.Lock()
	fakerInstance = gofakeit.New(uint64(time.Now().UnixNano() & 0x7FFFFFFFFFFFFFFF)) //nolint:gosec // masking ensures safe conversion
	fakerMutex.Unlock()
}

// ProcessMockTemplate processes the template string, using the shared Faker instance
// as the data context. This ensures that multiple calls to this function will
// draw from the same logical "sequence" of random data until ReshuffleMockData is called.
//
// tmplStr: The template string (e.g., "{{.FirstName}}", "{{.Email}}", etc).
func ProcessMockTemplate(tmplStr string, out UserOutput) (string, error) {
	// Step 1: Acquire the current faker instance
	fakerMutex.RLock()
	currentFaker := fakerInstance // Get the current faker instance for this operation
	fakerMutex.RUnlock()
	// Step 2: Process with gofakeit.Template, passing the Faker instance itself as Data.
	value, err := gofakeit.Template(tmplStr, &gofakeit.TemplateOptions{
		Data: currentFaker, // Pass the Faker instance for template field access
	})
	if err != nil {
		if out != nil {
			out.Errorln("MockData: mock template processing failed: %w", err)
			out.Errorln("See for more details: https://github.com/brianvoe/gofakeit", err)
		}
		return "", fmt.Errorf("mock template processing failed: %w", err)
	}
	return value, nil
}
