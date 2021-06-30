package accountinformation

import (
	"app/primitives"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/Rhymond/go-money"

	"github.com/almerlucke/go-iban/iban"
)

var parentUUID uuid.UUID

func init() {
	parentUUID, _ = uuid.FromBytes([]byte("25675de1-ea03-49b8-bc67-6153906134dd"))
}

type TransactionIDOrError struct {
	ID  *primitives.TransactionID
	err error
}

type TransactionIDFetcher interface {
	FetchID(payerIban *iban.IBAN, payeeIban *iban.IBAN, amount money.Money, description string, transactionDate time.Time, out chan<- TransactionIDOrError)
}

type InMemoryTransactionIDFetcher struct {
}

func NewInMemoryTransactionIDFetcher() *InMemoryTransactionIDFetcher {
	return &InMemoryTransactionIDFetcher{}
}

func (fetcher *InMemoryTransactionIDFetcher) FetchID(payerIban *iban.IBAN, payeeIban *iban.IBAN, amount money.Money, description string, transactionDate time.Time, out chan<- TransactionIDOrError) {
	defer close(out)

	var payerIbanString string
	if payerIban != nil {
		payerIbanString = payerIban.Code
	}

	var payeeIbanString string
	if payeeIban != nil {
		payeeIbanString = payeeIban.Code
	}

	amountString := strconv.FormatInt(amount.Amount(), 10)
	transactionDateString := transactionDate.Format("2006-01-02 15:04")

	data := payerIbanString + "-" + payeeIbanString + "-" + amountString + "-" + description + "-" + transactionDateString
	id := uuid.NewMD5(parentUUID, []byte(data))
	txID := primitives.TransactionID(id)

	out <- TransactionIDOrError{ID: &txID}
}
