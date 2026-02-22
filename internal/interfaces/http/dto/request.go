package dto

// CreateAccountRequest is the HTTP request body for creating an account.
type CreateAccountRequest struct {
	UserID         string  `json:"user_id" validate:"required"`
	InitialBalance float64 `json:"initial_balance" validate:"gte=0"`
	Currency       string  `json:"currency" validate:"required,len=3"`
}

// CreatePaymentRequest is the HTTP request body for creating a payment.
type CreatePaymentRequest struct {
	PaymentType          string  `json:"payment_type" validate:"required,oneof=internal_transfer external_payment"`
	SourceAccountID      string  `json:"source_account_id" validate:"required,uuid"`
	DestinationAccountID string  `json:"destination_account_id,omitempty" validate:"omitempty,uuid"`
	Amount               float64 `json:"amount" validate:"required,gt=0"`
	Currency             string  `json:"currency" validate:"required,len=3"`
	Provider             string  `json:"provider,omitempty" validate:"omitempty,oneof=stripe paypal"`
}

// TransferRequest is the HTTP request body for internal transfers.
type TransferRequest struct {
	SourceAccountID      string  `json:"source_account_id" validate:"required,uuid"`
	DestinationAccountID string  `json:"destination_account_id" validate:"required,uuid"`
	Amount               float64 `json:"amount" validate:"required,gt=0"`
	Currency             string  `json:"currency" validate:"required,len=3"`
}
