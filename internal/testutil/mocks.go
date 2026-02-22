package testutil

import (
	"context"
	"sync"

	"github.com/cassiomorais/payments/internal/domain/account"
	"github.com/cassiomorais/payments/internal/domain/outbox"
	"github.com/cassiomorais/payments/internal/domain/payment"
	"github.com/google/uuid"
)

// --- Payment Repository Mock ---

// MockPaymentRepository is a mock implementation of payment.Repository.
type MockPaymentRepository struct {
	mu       sync.Mutex
	payments map[uuid.UUID]*payment.Payment
	events   map[uuid.UUID][]*payment.PaymentEvent
	byKey    map[string]*payment.Payment

	CreateFunc              func(ctx context.Context, p *payment.Payment) error
	GetByIDFunc             func(ctx context.Context, id uuid.UUID) (*payment.Payment, error)
	GetByIdempotencyKeyFunc func(ctx context.Context, key string) (*payment.Payment, error)
	UpdateFunc              func(ctx context.Context, p *payment.Payment) error
	ListFunc                func(ctx context.Context, filter payment.ListFilter) ([]*payment.Payment, error)
	AddEventFunc            func(ctx context.Context, event *payment.PaymentEvent) error
	GetEventsFunc           func(ctx context.Context, paymentID uuid.UUID) ([]*payment.PaymentEvent, error)
}

func NewMockPaymentRepository() *MockPaymentRepository {
	return &MockPaymentRepository{
		payments: make(map[uuid.UUID]*payment.Payment),
		events:   make(map[uuid.UUID][]*payment.PaymentEvent),
		byKey:    make(map[string]*payment.Payment),
	}
}

func (m *MockPaymentRepository) Create(ctx context.Context, p *payment.Payment) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, p)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.payments[p.ID] = p
	m.byKey[p.IdempotencyKey] = p
	return nil
}

func (m *MockPaymentRepository) GetByID(ctx context.Context, id uuid.UUID) (*payment.Payment, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.payments[id]
	if !ok {
		return nil, nil
	}
	return p, nil
}

func (m *MockPaymentRepository) GetByIdempotencyKey(ctx context.Context, key string) (*payment.Payment, error) {
	if m.GetByIdempotencyKeyFunc != nil {
		return m.GetByIdempotencyKeyFunc(ctx, key)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.byKey[key]
	if !ok {
		return nil, nil
	}
	return p, nil
}

func (m *MockPaymentRepository) Update(ctx context.Context, p *payment.Payment) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, p)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.payments[p.ID] = p
	return nil
}

func (m *MockPaymentRepository) List(ctx context.Context, filter payment.ListFilter) ([]*payment.Payment, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx, filter)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*payment.Payment, 0, len(m.payments))
	for _, p := range m.payments {
		result = append(result, p)
	}
	return result, nil
}

func (m *MockPaymentRepository) AddEvent(ctx context.Context, event *payment.PaymentEvent) error {
	if m.AddEventFunc != nil {
		return m.AddEventFunc(ctx, event)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events[event.PaymentID] = append(m.events[event.PaymentID], event)
	return nil
}

func (m *MockPaymentRepository) GetEvents(ctx context.Context, paymentID uuid.UUID) ([]*payment.PaymentEvent, error) {
	if m.GetEventsFunc != nil {
		return m.GetEventsFunc(ctx, paymentID)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.events[paymentID], nil
}

// --- Account Repository Mock ---

// MockAccountRepository is a mock implementation of account.Repository.
type MockAccountRepository struct {
	mu           sync.Mutex
	accounts     map[uuid.UUID]*account.Account
	transactions map[uuid.UUID][]*account.Transaction

	CreateFunc          func(ctx context.Context, acct *account.Account) error
	GetByIDFunc         func(ctx context.Context, id uuid.UUID) (*account.Account, error)
	GetByUserIDFunc     func(ctx context.Context, userID string, currency string) (*account.Account, error)
	UpdateFunc          func(ctx context.Context, acct *account.Account) error
	AddTransactionFunc  func(ctx context.Context, tx *account.Transaction) error
	GetTransactionsFunc func(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]*account.Transaction, error)
	LockFunc            func(ctx context.Context, id uuid.UUID) (*account.Account, error)
}

func NewMockAccountRepository() *MockAccountRepository {
	return &MockAccountRepository{
		accounts:     make(map[uuid.UUID]*account.Account),
		transactions: make(map[uuid.UUID][]*account.Transaction),
	}
}

// AddAccount pre-populates the mock with an account.
func (m *MockAccountRepository) AddAccount(acct *account.Account) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accounts[acct.ID] = acct
}

func (m *MockAccountRepository) Create(ctx context.Context, acct *account.Account) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, acct)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accounts[acct.ID] = acct
	return nil
}

func (m *MockAccountRepository) GetByID(ctx context.Context, id uuid.UUID) (*account.Account, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	acct, ok := m.accounts[id]
	if !ok {
		return nil, nil
	}
	return acct, nil
}

func (m *MockAccountRepository) GetByUserID(ctx context.Context, userID string, currency string) (*account.Account, error) {
	if m.GetByUserIDFunc != nil {
		return m.GetByUserIDFunc(ctx, userID, currency)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, acct := range m.accounts {
		if acct.UserID == userID && acct.Currency == currency {
			return acct, nil
		}
	}
	return nil, nil
}

func (m *MockAccountRepository) Update(ctx context.Context, acct *account.Account) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, acct)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accounts[acct.ID] = acct
	return nil
}

func (m *MockAccountRepository) AddTransaction(ctx context.Context, tx *account.Transaction) error {
	if m.AddTransactionFunc != nil {
		return m.AddTransactionFunc(ctx, tx)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.transactions[tx.AccountID] = append(m.transactions[tx.AccountID], tx)
	return nil
}

func (m *MockAccountRepository) GetTransactions(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]*account.Transaction, error) {
	if m.GetTransactionsFunc != nil {
		return m.GetTransactionsFunc(ctx, accountID, limit, offset)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	txns := m.transactions[accountID]
	if offset >= len(txns) {
		return nil, nil
	}
	end := offset + limit
	if end > len(txns) {
		end = len(txns)
	}
	return txns[offset:end], nil
}

func (m *MockAccountRepository) Lock(ctx context.Context, id uuid.UUID) (*account.Account, error) {
	if m.LockFunc != nil {
		return m.LockFunc(ctx, id)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	acct, ok := m.accounts[id]
	if !ok {
		return nil, nil
	}
	return acct, nil
}

// GetAccountByID returns the stored account (test helper, no context needed).
func (m *MockAccountRepository) GetAccountByID(id uuid.UUID) *account.Account {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.accounts[id]
}

// --- Transaction Manager Mock ---

// MockTransactionManager is a mock implementation of TransactionManager.
type MockTransactionManager struct {
	WithTransactionFunc func(ctx context.Context, fn func(ctx context.Context) error) error
}

func NewMockTransactionManager() *MockTransactionManager {
	return &MockTransactionManager{}
}

func (m *MockTransactionManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if m.WithTransactionFunc != nil {
		return m.WithTransactionFunc(ctx, fn)
	}
	return fn(ctx)
}

// --- Outbox Repository Mock ---

// MockOutboxRepository is a mock implementation of outbox.Repository.
type MockOutboxRepository struct {
	InsertFunc        func(ctx context.Context, entry *outbox.Entry) error
	GetPendingFunc    func(ctx context.Context, limit int) ([]*outbox.Entry, error)
	MarkPublishedFunc func(ctx context.Context, id uuid.UUID) error
	MarkFailedFunc    func(ctx context.Context, id uuid.UUID) error
}

func (m *MockOutboxRepository) Insert(ctx context.Context, entry *outbox.Entry) error {
	if m.InsertFunc != nil {
		return m.InsertFunc(ctx, entry)
	}
	return nil
}

func (m *MockOutboxRepository) GetPending(ctx context.Context, limit int) ([]*outbox.Entry, error) {
	if m.GetPendingFunc != nil {
		return m.GetPendingFunc(ctx, limit)
	}
	return nil, nil
}

func (m *MockOutboxRepository) MarkPublished(ctx context.Context, id uuid.UUID) error {
	if m.MarkPublishedFunc != nil {
		return m.MarkPublishedFunc(ctx, id)
	}
	return nil
}

func (m *MockOutboxRepository) MarkFailed(ctx context.Context, id uuid.UUID) error {
	if m.MarkFailedFunc != nil {
		return m.MarkFailedFunc(ctx, id)
	}
	return nil
}
