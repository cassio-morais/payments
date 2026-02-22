package outbox

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEntry(t *testing.T) {
	aggregateID := uuid.New()
	payload := map[string]any{
		"payment_id":   "pay_123",
		"amount_cents": 10000,
		"currency":     "USD",
	}

	entry := NewEntry("payment", aggregateID, "payment.created", payload)

	require.NotNil(t, entry)
	assert.NotEqual(t, uuid.Nil, entry.ID)
	assert.Equal(t, "payment", entry.AggregateType)
	assert.Equal(t, aggregateID, entry.AggregateID)
	assert.Equal(t, "payment.created", entry.EventType)
	assert.Equal(t, payload, entry.Payload)
	assert.Equal(t, StatusPending, entry.Status)
	assert.Equal(t, 0, entry.RetryCount)
	assert.Equal(t, 5, entry.MaxRetries)
	assert.False(t, entry.CreatedAt.IsZero())
	assert.Nil(t, entry.PublishedAt)
}

func TestNewEntry_EmptyPayload(t *testing.T) {
	aggregateID := uuid.New()
	entry := NewEntry("account", aggregateID, "account.created", nil)

	require.NotNil(t, entry)
	assert.Nil(t, entry.Payload)
	assert.Equal(t, StatusPending, entry.Status)
}

func TestNewEntry_DifferentEventTypes(t *testing.T) {
	aggregateID := uuid.New()

	tests := []struct {
		name          string
		aggregateType string
		eventType     string
	}{
		{"payment created", "payment", "payment.created"},
		{"payment completed", "payment", "payment.completed"},
		{"payment failed", "payment", "payment.failed"},
		{"account created", "account", "account.created"},
		{"account suspended", "account", "account.suspended"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := NewEntry(tt.aggregateType, aggregateID, tt.eventType, nil)
			assert.Equal(t, tt.aggregateType, entry.AggregateType)
			assert.Equal(t, tt.eventType, entry.EventType)
		})
	}
}

func TestStatus_Constants(t *testing.T) {
	assert.Equal(t, Status("pending"), StatusPending)
	assert.Equal(t, Status("published"), StatusPublished)
	assert.Equal(t, Status("failed"), StatusFailed)
}

func TestEntry_UniqueIDs(t *testing.T) {
	aggregateID := uuid.New()
	entry1 := NewEntry("payment", aggregateID, "payment.created", nil)
	entry2 := NewEntry("payment", aggregateID, "payment.created", nil)

	// Each entry should have a unique ID even with same aggregate
	assert.NotEqual(t, entry1.ID, entry2.ID)
	assert.Equal(t, entry1.AggregateID, entry2.AggregateID)
}

func TestEntry_PayloadTypes(t *testing.T) {
	aggregateID := uuid.New()

	tests := []struct {
		name    string
		payload map[string]any
	}{
		{
			name: "numeric values",
			payload: map[string]any{
				"amount": 100,
				"count":  5,
			},
		},
		{
			name: "string values",
			payload: map[string]any{
				"status":   "active",
				"currency": "USD",
			},
		},
		{
			name: "mixed types",
			payload: map[string]any{
				"id":      uuid.New().String(),
				"amount":  10000,
				"active":  true,
				"details": map[string]string{"key": "value"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := NewEntry("test", aggregateID, "test.event", tt.payload)
			assert.Equal(t, tt.payload, entry.Payload)
		})
	}
}
