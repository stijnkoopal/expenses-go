package bunqconnector

import (
	"context"
	"math"
	"strconv"
	"time"

	"github.com/rickb777/date/period"

	"github.com/almerlucke/go-iban/iban"

	"github.com/OGKevin/go-bunq/bunq"
	"github.com/Rhymond/go-money"
)

type bunqAccountID int
type bunqTransactionID int
type bunqDirectDebitTransactionID int
type bunqScheduleID int

type apiAuth struct{}

type bunqAPI interface {
	fetchAccounts(ctx context.Context, out chan<- apiAccountOrError)
	fetchTransactions(ctx context.Context, bunqAccountID bunqAccountID, newerThan time.Time, out chan<- apiTransactionOrError)
	fetchDirectDebitTransactions(ctx context.Context, bunqAccountID bunqAccountID, out chan<- apiDirectDebitTransactionOrError)
	fetchSchedules(ctx context.Context, bunqAccountID bunqAccountID, out chan<- apiScheduleOrError)
}

type realBunqAPI struct {
	client      *bunq.Client
	rateLimiter rateLimiter
}

func newRealBunqAPI(rateLimiter rateLimiter, client *bunq.Client) realBunqAPI {
	return realBunqAPI{rateLimiter: rateLimiter, client: client}
}

type apiAccount struct {
	bunqAccountID  bunqAccountID
	iban           iban.IBAN
	joint          bool
	alias          string
	balance        money.Money
	fetchTimestamp time.Time
}

type apiAccountOrError struct {
	apiAccount
	err error
}

func (api realBunqAPI) fetchAccounts(ctx context.Context, out chan<- apiAccountOrError) {
	defer func() { close(out) }()

	bankChannel := make(chan apiAccountOrError, 25)
	savingsChannel := make(chan apiAccountOrError, 25)

	go api.fetchBankAccounts(ctx, bankChannel)
	go api.fetchSavingAccounts(ctx, savingsChannel)

	for {
		select {
		case bankAccount, ok := <-bankChannel:
			if !ok {
				bankChannel = nil
			} else {
				out <- bankAccount
			}

		case savingsAccount, ok := <-savingsChannel:
			if !ok {
				savingsChannel = nil
			} else {
				out <- savingsAccount
			}
		}

		if bankChannel == nil && savingsChannel == nil {
			break
		}
	}
}

type apiTransactionGeolocation struct {
	latitude  float64
	longitude float64
	altitude  float64
	radius    float64
}

type apiTransaction struct {
	bunqTransactionID     bunqTransactionID
	amount                money.Money
	name                  *string
	iban                  *iban.IBAN
	counterpartyName      *string
	counterpartyIban      *iban.IBAN
	description           string
	institutionScheduleID *string
	balanceAfterMutation  money.Money
	geolocation           *apiTransactionGeolocation
	transactionDate       time.Time
	fetchTimestamp        time.Time
}

type apiTransactionOrError struct {
	apiTransaction
	err error
}

func (api realBunqAPI) fetchTransactions(ctx context.Context, bunqAccountID bunqAccountID, newerThan time.Time, out chan<- apiTransactionOrError) {
	defer func() { close(out) }()

	var pagination *bunq.Pagination

	for {
		select {
		case <-ctx.Done():
			break
		case <-api.rateLimiter.forGet():
			var response *bunq.ResponsePaymentGet
			var err error

			if pagination == nil {
				response, err = api.client.PaymentService.GetAllPayment(uint(bunqAccountID))
			} else {
				response, err = api.client.PaymentService.GetAllOlderPayment(response.Pagination)
			}

			if err != nil {
				out <- apiTransactionOrError{err: err}
				break
			}

			if len(response.Response) == 0 {
				break
			}

			hasNewerTransaction := false
			for _, tx := range response.Response {
				mapped, err := mapTransaction(tx.Payment)
				if err != nil {
					out <- apiTransactionOrError{err: err}
				} else if isPaymentNewerThan(tx.Payment, newerThan) {
					hasNewerTransaction = true
					out <- apiTransactionOrError{apiTransaction: *mapped}
				}
			}

			if !hasNewerTransaction {
				break
			}
		}
	}
}

type apiDirectDebitTransaction struct {
	bunqDirectDebitTransactionID bunqDirectDebitTransactionID
	amount                       money.Money
	name                         string
	iban                         iban.IBAN
	counterpartyName             string
	counterpartyIBAN             iban.IBAN
	description                  string
	creditSchemeID               string
	mandateID                    string
	created                      time.Time
	responsed                    *time.Time
	fetchTimestamp               time.Time
}

type apiDirectDebitTransactionOrError struct {
	apiDirectDebitTransaction
	err error
}

func (api realBunqAPI) fetchDirectDebitTransactions(ctx context.Context, bunqAccountID bunqAccountID, newerThan time.Time, out chan<- apiDirectDebitTransactionOrError) {
	defer func() { close(out) }()

	var pagination *bunq.Pagination

	for {
		select {
		case <-ctx.Done():
			break
		case <-api.rateLimiter.forGet():
			var response *bunq.ResponseRequestResponsesGet
			var err error

			if pagination == nil {
				response, err = api.client.RequestResponseService.GetAllRequestResponses(uint(bunqAccountID))
			} else {
				response, err = api.client.RequestResponseService.GetAllOlderRequestResponses(response.Pagination)
			}

			if err != nil {
				out <- apiDirectDebitTransactionOrError{err: err}
				break
			}

			if len(response.Response) == 0 {
				break
			}

			hasNewer := false
			for _, tx := range response.Response {
				mapped, err := mapRequestResponseToDirectDebitTransaction(tx.RequestResponse)
				if err != nil {
					out <- apiDirectDebitTransactionOrError{err: err}
				} else if isRequestResponseNewerThan(tx.RequestResponse, newerThan) {
					hasNewer = true
					out <- apiDirectDebitTransactionOrError{apiDirectDebitTransaction: *mapped}
				}
			}

			if !hasNewer {
				break
			}
		}
	}
}

type apiSchedule struct {
	bunqScheduleID   bunqScheduleID
	startDate        time.Time
	endDate          *time.Time
	frequency        period.Period
	amount           money.Money
	description      string
	name             string
	iban             iban.IBAN
	counterpartyName string
	counterpartyIBAN iban.IBAN
	fetchTimestamp   time.Time
}

type apiScheduleOrError struct {
	apiSchedule
	err error
}

func (api realBunqAPI) fetchSchedules(ctx context.Context, bunqAccountID bunqAccountID, out chan<- apiScheduleOrError) {
	defer func() { close(out) }()

	select {
	case <-ctx.Done():
		return
	case <-api.rateLimiter.forGet():
		resp, err := api.client.ScheduledPaymentService.GetAllScheduledPayments(int(bunqAccountID))

		if err != nil {
			out <- apiScheduleOrError{err: err}
		} else {
			for _, a := range resp.Response {
				mapped, err := mapSchedule(a.ScheduledPayment)
				if err != nil {
					out <- apiScheduleOrError{err: err}
				} else {
					out <- apiScheduleOrError{apiSchedule: *mapped}
				}
			}
		}
	}
}

func (api realBunqAPI) fetchBankAccounts(ctx context.Context, out chan<- apiAccountOrError) {
	defer func() { close(out) }()

	select {
	case <-ctx.Done():
		return
	case <-api.rateLimiter.forGet():
		resp, err := api.client.AccountService.GetAllMonetaryAccountBank()
		if err != nil {
			out <- apiAccountOrError{err: err}
		} else {
			for _, a := range resp.Response {
				mapped, err := mapBankToAccount(a.MonetaryAccountBank)
				if err != nil {
					out <- apiAccountOrError{err: err}
				} else {
					out <- apiAccountOrError{apiAccount: *mapped}
				}
			}
		}
	}
}

func (api realBunqAPI) fetchSavingAccounts(ctx context.Context, out chan<- apiAccountOrError) {
	defer func() { close(out) }()

	select {
	case <-ctx.Done():
		return
	case <-api.rateLimiter.forGet():
		resp, err := api.client.AccountService.GetAllMonetaryAccountSaving()
		if err != nil {
			out <- apiAccountOrError{err: err}
		} else {
			for _, a := range resp.Response {
				mapped, err := mapSavingsToAccount(a.MonetaryAccountSaving)
				if err != nil {
					out <- apiAccountOrError{err: err}
				} else {
					out <- apiAccountOrError{apiAccount: *mapped}
				}
			}
		}
	}
}

func mapBankToAccount(account bunq.MonetaryAccountBank) (*apiAccount, error) {
	balance, err := mapAmount(account.Balance)

	if err != nil {
		return nil, err
	}

	iban, err := iban.NewIBAN(account.GetIBANPointer().Value)

	if err != nil {
		return nil, err
	}

	return &apiAccount{
		bunqAccountID:  bunqAccountID(account.ID),
		iban:           *iban,
		joint:          false,
		alias:          account.Description,
		balance:        *balance,
		fetchTimestamp: time.Now(),
	}, nil
}

func mapSavingsToAccount(account bunq.MonetaryAccountSaving) (*apiAccount, error) {
	balance, err := mapAmount(account.Balance)

	if err != nil {
		return nil, err
	}

	iban, err := iban.NewIBAN(account.GetIBANPointer().Value)

	if err != nil {
		return nil, err
	}

	return &apiAccount{
		bunqAccountID:  bunqAccountID(account.ID),
		iban:           *iban,
		joint:          false,
		alias:          account.Description,
		balance:        *balance,
		fetchTimestamp: time.Now(),
	}, nil
}

func mapTransaction(tx bunq.Payment) (*apiTransaction, error) {
	amount, err := mapAmount(tx.Amount)

	if err != nil {
		return nil, err
	}

	balanceAfterMutation, err := mapAmount(tx.BalanceAfterMutation)

	if err != nil {
		return nil, err
	}

	transactionDate, err := parseBunqDateTime(tx.Created)

	if err != nil {
		return nil, err
	}

	var scheduleID *string = nil
	if tx.ScheduledID > 0 {
		x := strconv.Itoa(tx.ScheduledID)
		scheduleID = &x
	}

	geolocation := &apiTransactionGeolocation{
		latitude:  tx.Geolocation.Latitude,
		longitude: tx.Geolocation.Longitude,
		altitude:  tx.Geolocation.Altitude,
		radius:    tx.Geolocation.Radius,
	}

	aliasIban, _ := iban.NewIBAN(tx.Alias.IBAN)
	counterpartyIban, _ := iban.NewIBAN(tx.CounterpartyAlias.IBAN)

	return &apiTransaction{
		bunqTransactionID:     bunqTransactionID(tx.ID),
		amount:                *amount,
		name:                  getNameFrom(tx.Alias),
		iban:                  aliasIban,
		counterpartyName:      getNameFrom(tx.CounterpartyAlias),
		counterpartyIban:      counterpartyIban,
		description:           tx.Description,
		institutionScheduleID: scheduleID,
		balanceAfterMutation:  *balanceAfterMutation,
		geolocation:           geolocation,
		transactionDate:       transactionDate,
		fetchTimestamp:        time.Now(),
	}, nil
}

func mapSchedule(schedule bunq.ScheduledPayment) (*apiSchedule, error) {
	amount, err := mapAmount(schedule.Payment.Amount)

	if err != nil {
		return nil, err
	}

	startDate, err := parseBunqDateTime(schedule.Schedule.TimeStart)

	if err != nil {
		return nil, err
	}

	endDate, err := parseOptionalBunqDateTime(schedule.Schedule.TimeEnd)

	aliasIban, _ := iban.NewIBAN(schedule.Payment.Alias.IBAN)
	counterpartyIban, _ := iban.NewIBAN(schedule.Payment.CounterpartyAlias.IBAN)

	if schedule.Schedule.RecurrenceUnit == "ONCE" {
	}
	return &apiSchedule{
		bunqScheduleID:   bunqScheduleID(schedule.ID),
		startDate:        startDate,
		endDate:          endDate,
		frequency:        mapToPeriod(schedule.Schedule.RecurrenceUnit, schedule.Schedule.RecurrenceSize),
		amount:           *amount,
		description:      schedule.Payment.Description,
		name:             schedule.Payment.Alias.DisplayName,
		iban:             *aliasIban,
		counterpartyName: schedule.Payment.CounterpartyAlias.DisplayName,
		counterpartyIBAN: *counterpartyIban,
		fetchTimestamp:   time.Now(),
	}, nil
}

func mapRequestResponseToDirectDebitTransaction(rr bunq.RequestResponse) (*apiDirectDebitTransaction, error) {
	amount, err := mapAmount(rr.Amount)

	if err != nil {
		return nil, err
	}

	created, err := parseBunqDateTime(rr.Created)

	if err != nil {
		return nil, err
	}

	responded, err := parseOptionalBunqDateTime(rr.Responded)

	if err != nil {
		return nil, err
	}

	aliasIban, _ := iban.NewIBAN(rr.Alias.IBAN)
	counterpartyIban, _ := iban.NewIBAN(rr.CounterpartyAlias.IBAN)

	return &apiDirectDebitTransaction{
		bunqDirectDebitTransactionID: bunqDirectDebitTransactionID(rr.ID),
		amount:                       *amount,
		name:                         rr.Alias.DisplayName,
		iban:                         *aliasIban,
		counterpartyName:             rr.CounterpartyAlias.DisplayName,
		counterpartyIBAN:             *counterpartyIban,
		description:                  rr.Description,
		creditSchemeID:               rr.CreditSchemeID,
		mandateID:                    rr.MandateID,
		created:                      created,
		responsed:                    responded,
		fetchTimestamp:               time.Now(),
	}, nil
}

func mapAmount(amount bunq.Amount) (*money.Money, error) {
	balance, err := strconv.ParseFloat(amount.Value, 32)

	if err != nil {
		return nil, err
	}
	return money.New(int64(math.Round(balance*100)), amount.Currency), nil
}

func isPaymentNewerThan(payment bunq.Payment, newerThan time.Time) bool {
	created, err := parseBunqDateTime(payment.Created)
	if err != nil {
		panic(err)
	} else {
		return created.After(newerThan)
	}
}

func isRequestResponseNewerThan(requestResponse bunq.RequestResponse, newerThan time.Time) bool {
	created, err := parseBunqDateTime(requestResponse.Created)
	if err != nil {
		panic(err)
	} else {
		return created.After(newerThan)
	}
}

func parseBunqDateTime(input string) (time.Time, error) {
	return time.Parse("2006-01-02 15:04:05.000000", input)
}

func parseOptionalBunqDateTime(input string) (*time.Time, error) {
	if input == "" {
		return nil, nil
	}
	x, err := time.Parse("2006-01-02 15:04:05.000000", input)

	if err != nil {
		return nil, err
	}
	return &x, err
}

func getNameFrom(account bunq.LabelMonetaryAccount) *string {
	if len(account.DisplayName) > 0 {
		return &account.DisplayName
	} else if len(account.LabelUser.DisplayName) > 0 {
		return &account.LabelUser.DisplayName
	} else if len(account.LabelUser.PublicNickName) > 0 {
		return &account.LabelUser.PublicNickName
	}
	return nil
}

func mapToPeriod(recurrenceUnit string, recurrenceSize int) period.Period {
	switch recurrenceUnit {
	case "HOURLY":
		return period.NewHMS(recurrenceSize, 0, 0)
	case "DAILY":
		return period.NewYMD(0, 0, recurrenceSize)
	case "WEEKLY":
		return period.NewYMD(0, 0, 7*recurrenceSize)
	case "MONTHLY":
		return period.NewYMD(0, recurrenceSize, 0)
	case "YEARLY":
		return period.NewYMD(recurrenceSize, 0, 0)
	}
	return period.NewYMD(0, recurrenceSize, 0)
}
