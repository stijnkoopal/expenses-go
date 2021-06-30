package recurring

import (
	"app/primitives"

	"github.com/google/uuid"
)

var parentUUID uuid.UUID

func init() {
	parentUUID, _ = uuid.FromBytes([]byte("17675de1-ea03-19b8-1c67-4153906134f1"))
}

type RecurringTransactionIDOrError struct {
	ID  *primitives.RecurringTransactionID
	err error
}

type RecurringTransactionInstanceIDOrError struct {
	ID  *primitives.RecurringTransactionInstanceID
	err error
}

type RecurringTransactionIDFetcher interface {
	FetchID(institution primitives.Institution, institutionEntityID string, out chan<- RecurringTransactionIDOrError)
}

type RecurringTransactionInstanceIDFetcher interface {
	FetchID(institution primitives.Institution, institutionEntityID string, out chan<- RecurringTransactionInstanceIDOrError)
}

// TODO:
// type InMemoryTransactionIDFetcher struct {
// }

// func NewInMemoryTransactionIDFetcher() *InMemoryTransactionIDFetcher {
// 	return &InMemoryTransactionIDFetcher{}
// }

// func (fetcher *InMemoryTransactionIDFetcher) FetchID(payerIban *iban.IBAN, payeeIban *iban.IBAN, amount money.Money, description string, transactionDate time.Time, out chan<- TransactionIDOrError) {
// 	defer close(out)

// 	var payerIbanString string
// 	if payerIban != nil {
// 		payerIbanString = payerIban.Code
// 	}

// 	var payeeIbanString string
// 	if payeeIban != nil {
// 		payeeIbanString = payeeIban.Code
// 	}

// 	amountString := strconv.FormatInt(amount.Amount(), 10)
// 	transactionDateString := transactionDate.Format("2006-01-02 15:04")

// 	data := payerIbanString + "-" + payeeIbanString + "-" + amountString + "-" + description + "-" + transactionDateString
// 	id := uuid.NewMD5(parentUUID, []byte(data))
// 	txID := primitives.TransactionID(id)

// 	out <- TransactionIDOrError{ID: &txID}
// }
