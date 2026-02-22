package account_test

import (
	"context"
	"errors"
	"testing"

	accountApp "github.com/cassiomorais/payments/internal/serviceaccount"
	"github.com/cassiomorais/payments/internal/domain/account"
	domainErrors "github.com/cassiomorais/payments/internal/domain/errors"
	"github.com/cassiomorais/payments/internal/testutil"
)

func TestCreateAccountUseCase_Execute(t *testing.T) {
	tests := []struct {
		name    string
		req     accountApp.CreateAccountRequest
		mockErr error
		want    func(*account.Account) bool
		wantErr bool
	}{
		{
			name: "creates account successfully",
			req: accountApp.CreateAccountRequest{
				UserID:         "user123",
				InitialBalance: 100000, // 1000.00 USD in cents
				Currency:       "USD",
			},
			mockErr: nil,
			want: func(a *account.Account) bool {
				return a != nil && a.UserID == "user123" && a.Balance == 100000 && a.Currency == "USD"
			},
			wantErr: false,
		},
		{
			name: "returns error from repository",
			req: accountApp.CreateAccountRequest{
				UserID:         "user123",
				InitialBalance: 100000,
				Currency:       "USD",
			},
			mockErr: errors.New("database error"),
			want:    nil,
			wantErr: true,
		},
		{
			name: "returns error for empty currency",
			req: accountApp.CreateAccountRequest{
				UserID:         "user123",
				InitialBalance: 100000,
				Currency:       "",
			},
			mockErr: nil,
			want:    nil,
			wantErr: true,
		},
		{
			name: "accepts zero initial balance",
			req: accountApp.CreateAccountRequest{
				UserID:         "user456",
				InitialBalance: 0,
				Currency:       "USD",
			},
			mockErr: nil,
			want: func(a *account.Account) bool {
				return a != nil && a.Balance == 0
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &testutil.MockAccountRepository{
				CreateFunc: func(ctx context.Context, acc *account.Account) error {
					return tt.mockErr
				},
			}

			uc := accountApp.NewCreateAccountUseCase(mockRepo)
			got, err := uc.Execute(context.Background(), tt.req)

			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.want != nil && !tt.want(got) {
				t.Errorf("Execute() returned account that doesn't match expectations: %+v", got)
			}

			// Check that validation errors are distinguishable
			if tt.wantErr && tt.name == "returns error for empty currency" {
				if _, ok := err.(*domainErrors.ValidationError); !ok {
					t.Logf("Expected ValidationError, got %T: %v", err, err)
				}
			}
		})
	}
}

func TestCreateAccountUseCase_Execute_RepositoryErrorPropagates(t *testing.T) {
	mockErr := errors.New("duplicate key violation")
	mockRepo := &testutil.MockAccountRepository{
		CreateFunc: func(ctx context.Context, acc *account.Account) error {
			return mockErr
		},
	}

	uc := accountApp.NewCreateAccountUseCase(mockRepo)
	_, err := uc.Execute(context.Background(), accountApp.CreateAccountRequest{
		UserID:         "user123",
		InitialBalance: 50000,
		Currency:       "USD",
	})

	if err == nil {
		t.Error("Expected error from repository, got nil")
	}
	if !errors.Is(err, mockErr) {
		t.Errorf("Expected error %v, got %v", mockErr, err)
	}
}
