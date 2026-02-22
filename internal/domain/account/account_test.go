package account

import (
	"testing"

	"github.com/cassiomorais/payments/internal/domain/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAccount_Valid(t *testing.T) {
	acct, err := NewAccount("user1", 100000, "USD")
	require.NoError(t, err)
	assert.Equal(t, "user1", acct.UserID)
	assert.Equal(t, int64(100000), acct.Balance)
	assert.Equal(t, "USD", acct.Currency)
	assert.Equal(t, 0, acct.Version)
	assert.Equal(t, StatusActive, acct.Status)
}

func TestNewAccount_ZeroBalance(t *testing.T) {
	acct, err := NewAccount("user1", 0, "USD")
	require.NoError(t, err)
	assert.Equal(t, int64(0), acct.Balance)
}

func TestNewAccount_NegativeBalance(t *testing.T) {
	_, err := NewAccount("user1", -1000, "USD")
	assert.Error(t, err)
}

func TestNewAccount_EmptyUserID(t *testing.T) {
	_, err := NewAccount("", 10000, "USD")
	assert.Error(t, err)
}

func TestNewAccount_EmptyCurrency(t *testing.T) {
	_, err := NewAccount("user1", 10000, "")
	assert.Error(t, err)
}

// --- Debit ---

func TestDebit_Success(t *testing.T) {
	acct, _ := NewAccount("user1", 50000, "USD")
	initialVersion := acct.Version

	err := acct.Debit(10000)
	require.NoError(t, err)
	assert.Equal(t, int64(40000), acct.Balance)
	assert.Equal(t, initialVersion+1, acct.Version)
}

func TestDebit_InsufficientFunds(t *testing.T) {
	acct, _ := NewAccount("user1", 5000, "USD")
	err := acct.Debit(10000)
	assert.ErrorIs(t, err, errors.ErrInsufficientFunds)
	assert.Equal(t, int64(5000), acct.Balance) // balance unchanged
}

func TestDebit_ExactBalance(t *testing.T) {
	acct, _ := NewAccount("user1", 10000, "USD")
	err := acct.Debit(10000)
	require.NoError(t, err)
	assert.Equal(t, int64(0), acct.Balance)
}

func TestDebit_ZeroAmount(t *testing.T) {
	acct, _ := NewAccount("user1", 10000, "USD")
	err := acct.Debit(0)
	assert.Error(t, err)
}

func TestDebit_NegativeAmount(t *testing.T) {
	acct, _ := NewAccount("user1", 10000, "USD")
	err := acct.Debit(-1000)
	assert.Error(t, err)
}

func TestDebit_InactiveAccount(t *testing.T) {
	acct, _ := NewAccount("user1", 10000, "USD")
	acct.Deactivate()
	err := acct.Debit(1000)
	assert.ErrorIs(t, err, errors.ErrAccountInactive)
}

// --- Credit ---

func TestCredit_Success(t *testing.T) {
	acct, _ := NewAccount("user1", 10000, "USD")
	initialVersion := acct.Version

	err := acct.Credit(5000)
	require.NoError(t, err)
	assert.Equal(t, int64(15000), acct.Balance)
	assert.Equal(t, initialVersion+1, acct.Version)
}

func TestCredit_ZeroAmount(t *testing.T) {
	acct, _ := NewAccount("user1", 10000, "USD")
	err := acct.Credit(0)
	assert.Error(t, err)
}

func TestCredit_InactiveAccount(t *testing.T) {
	acct, _ := NewAccount("user1", 10000, "USD")
	acct.Deactivate()
	err := acct.Credit(1000)
	assert.ErrorIs(t, err, errors.ErrAccountInactive)
}

// --- Status ---

func TestSuspend(t *testing.T) {
	acct, _ := NewAccount("user1", 10000, "USD")
	require.NoError(t, acct.Suspend())
	assert.Equal(t, StatusSuspended, acct.Status)
}

func TestActivate(t *testing.T) {
	acct, _ := NewAccount("user1", 10000, "USD")
	acct.Suspend()
	require.NoError(t, acct.Activate())
	assert.Equal(t, StatusActive, acct.Status)
}

func TestDeactivate(t *testing.T) {
	acct, _ := NewAccount("user1", 10000, "USD")
	require.NoError(t, acct.Deactivate())
	assert.Equal(t, StatusInactive, acct.Status)
}

// --- Version increments ---

func TestVersionIncrementsOnDebitAndCredit(t *testing.T) {
	acct, _ := NewAccount("user1", 100000, "USD")
	assert.Equal(t, 0, acct.Version)

	acct.Debit(10000)
	assert.Equal(t, 1, acct.Version)

	acct.Credit(5000)
	assert.Equal(t, 2, acct.Version)

	acct.Debit(20000)
	assert.Equal(t, 3, acct.Version)
}
