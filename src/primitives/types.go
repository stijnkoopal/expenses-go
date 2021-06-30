package primitives

import (
	"github.com/Rhymond/go-money"
	"github.com/google/uuid"
)

type Source string

const (
	DirectDebit Source = "DirectDebit"
	Schedule    Source = "Schedule"
)

type RecurringTransactionID uuid.UUID

func (recurringTransactionID RecurringTransactionID) String() string {
	return uuid.UUID(recurringTransactionID).String()
}

type RecurringTransactionInstanceID uuid.UUID

func (recurringTransactionInstanceID RecurringTransactionInstanceID) String() string {
	return uuid.UUID(recurringTransactionInstanceID).String()
}

type UserID uuid.UUID

func (userID UserID) String() string {
	return uuid.UUID(userID).String()
}

// Institution enum
type Institution string

// Institution enum
const (
	Bunq Institution = "Bunq"
)

type TransactionID uuid.UUID

func (transactionId TransactionID) String() string {
	return uuid.UUID(transactionId).String()
}

// MonetaryAccountID is the type of the id
type MonetaryAccountID uuid.UUID

func (monetaryAccountID MonetaryAccountID) String() string {
	return uuid.UUID(monetaryAccountID).String()
}

type SyncID uuid.UUID

func (syncID SyncID) String() string {
	return uuid.UUID(syncID).String()
}

type MoneyForCommand struct {
	Amount       int64
	CurrencyCode string
}

func (m MoneyForCommand) ToMoney() money.Money {
	return *money.New(m.Amount, m.CurrencyCode)
}

func NewMoneyForCommand(m money.Money) MoneyForCommand {
	return MoneyForCommand{Amount: m.Amount(), CurrencyCode: m.Currency().Code}
}
