package bus

import (
	"app/primitives"
	"time"

	"github.com/Rhymond/go-money"

	"github.com/almerlucke/go-iban/iban"
	"github.com/rickb777/date/period"
)

type Update interface{}

type StartRefreshUpdate struct {
	UserID              primitives.UserID
	SyncID              primitives.SyncID
	InstitutionEntityID string
	Started             time.Time
}

type DoneRefreshingUpdate struct {
	UserID                primitives.UserID
	InstititutionEntityID string
	SyncID                primitives.SyncID
	Started               time.Time
	Finished              time.Time
}

func NewDoneRefreshingUpdateFrom(startUpdate StartRefreshUpdate) DoneRefreshingUpdate {
	return DoneRefreshingUpdate{
		UserID:                startUpdate.UserID,
		InstititutionEntityID: startUpdate.InstitutionEntityID,
		SyncID:                startUpdate.SyncID,
		Started:               startUpdate.Started,
		Finished:              time.Now(),
	}
}

type Geolocation struct {
	Latitude  float64
	Longitude float64
	Altitude  float64
	Radius    float64
}

type TransactionDocument struct {
	Amount money.Money

	FromName                *string
	FromIBAN                *iban.IBAN
	FromInstition           *primitives.Institution
	FromInstitutionEntityID *string

	ToName              *string
	ToIBAN              *iban.IBAN
	ToInstition         *primitives.Institution
	ToInstitionEntityID *string

	Description           string
	InstitutionScheduleID *string
	BalanceAfterMutation  money.Money
	Geolocation           *Geolocation
	TransactionDate       time.Time
	FetchTimestamp        time.Time
}

type MonetaryAccountDocument struct {
	Iban                iban.IBAN
	Joint               bool
	OwnerUserID         primitives.UserID
	Alias               string
	Institution         primitives.Institution
	InstitutionEntityID string
	Balance             money.Money
	FetchTimestamp      time.Time
}

type ScheduleDocument struct {
	Institution         primitives.Institution
	InstitutionEntityID string
	FromIBAN            iban.IBAN
	FromName            string
	ToIBAN              iban.IBAN
	ToName              string
	Frequency           period.Period
	StartDate           time.Time
	EndDate             *time.Time
	Amount              money.Money
	Description         string
	FetchTimestamp      time.Time
}

type DirectDebitTransactionDocument struct {
	Institution         primitives.Institution
	InstititionEntityID string
	FromIBAN            iban.IBAN
	FromName            string
	ToIBAN              *iban.IBAN
	ToName              *string
	Description         string
	CreditSchemeID      string
	MandateID           string
	TransactionDate     time.Time
	Amount              money.Money
	FetchTimestamp      time.Time
}
