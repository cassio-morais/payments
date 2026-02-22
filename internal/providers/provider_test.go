package providers

import (
	"testing"

	"github.com/cassiomorais/payments/internal/domain/payment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFactory_WithDefaultProviders(t *testing.T) {
	factory := NewFactory()

	assert.NotNil(t, factory)
	assert.Len(t, factory.providers, 2) // stripe and paypal
	assert.Len(t, factory.circuitBreakers, 2)
}

func TestNewFactory_WithCustomProviders(t *testing.T) {
	mockProvider := NewMockProvider("test-provider")
	factory := NewFactory(mockProvider)

	assert.NotNil(t, factory)
	assert.Len(t, factory.providers, 1)
	assert.Contains(t, factory.providers, "test-provider")
}

func TestFactory_Get_StripeProvider(t *testing.T) {
	factory := NewFactory()

	provider, breaker, err := factory.Get(payment.ProviderStripe)
	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.NotNil(t, breaker)
	assert.Equal(t, "stripe", provider.Name())
}

func TestFactory_Get_PayPalProvider(t *testing.T) {
	factory := NewFactory()

	provider, breaker, err := factory.Get(payment.ProviderPayPal)
	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.NotNil(t, breaker)
	assert.Equal(t, "paypal", provider.Name())
}

func TestFactory_Get_UnknownProvider_Error(t *testing.T) {
	factory := NewFactory()

	provider, breaker, err := factory.Get(payment.Provider("unknown"))
	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Nil(t, breaker)
	assert.Contains(t, err.Error(), "unknown provider")
}

func TestFactory_Register(t *testing.T) {
	factory := NewFactory()
	mockProvider := NewMockProvider("custom")

	factory.Register(mockProvider)

	assert.Contains(t, factory.providers, "custom")
	assert.Contains(t, factory.circuitBreakers, "custom")

	provider, breaker, err := factory.Get(payment.Provider("custom"))
	require.NoError(t, err)
	assert.Equal(t, "custom", provider.Name())
	assert.NotNil(t, breaker)
}
