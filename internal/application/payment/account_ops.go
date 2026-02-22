package payment

import (
	"context"
	"time"

	"github.com/cassiomorais/payments/internal/domain/account"
	"github.com/google/uuid"
)

// accountOps provides common account debit/credit operations used across use cases.
type accountOps struct {
	accountRepo account.Repository
}

// debitAccount locks the account, debits the amount, updates it, and records the transaction.
func (ops *accountOps) debitAccount(ctx context.Context, accountID uuid.UUID, paymentID uuid.UUID, amount int64, description string) (balanceAfter int64, err error) {
	acct, err := ops.accountRepo.Lock(ctx, accountID)
	if err != nil {
		return 0, err
	}
	if err := acct.Debit(amount); err != nil {
		return 0, err
	}
	if err := ops.accountRepo.Update(ctx, acct); err != nil {
		return 0, err
	}
	if err := ops.accountRepo.AddTransaction(ctx, &account.Transaction{
		ID: uuid.New(), AccountID: acct.ID, PaymentID: &paymentID,
		TransactionType: account.TransactionDebit, Amount: amount,
		BalanceAfter: acct.Balance, Description: description, CreatedAt: time.Now(),
	}); err != nil {
		return 0, err
	}
	return acct.Balance, nil
}

// creditAccount locks the account, credits the amount, updates it, and records the transaction.
func (ops *accountOps) creditAccount(ctx context.Context, accountID uuid.UUID, paymentID uuid.UUID, amount int64, description string) (balanceAfter int64, err error) {
	acct, err := ops.accountRepo.Lock(ctx, accountID)
	if err != nil {
		return 0, err
	}
	if err := acct.Credit(amount); err != nil {
		return 0, err
	}
	if err := ops.accountRepo.Update(ctx, acct); err != nil {
		return 0, err
	}
	if err := ops.accountRepo.AddTransaction(ctx, &account.Transaction{
		ID: uuid.New(), AccountID: acct.ID, PaymentID: &paymentID,
		TransactionType: account.TransactionCredit, Amount: amount,
		BalanceAfter: acct.Balance, Description: description, CreatedAt: time.Now(),
	}); err != nil {
		return 0, err
	}
	return acct.Balance, nil
}
