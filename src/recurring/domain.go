package recurring

import (
	"app/primitives"
	"app/utils"
	"time"

	"github.com/Rhymond/go-money"
	"github.com/rickar/cal/v2/nl"

	"github.com/almerlucke/go-iban/iban"
	"github.com/jinzhu/copier"

	"github.com/jinzhu/now"
	"github.com/rickar/cal/v2"
	"github.com/rickb777/date/period"
)

type Status string

// Status enum
const (
	Active Status = "Active"
	Ended  Status = "Ended"
)

// TransactionParty party of a transaction
type TransactionParty struct {
	IBAN    iban.IBAN
	HasIBAN bool
	Name    string
	HasName bool
}

// NewTransactionParty constructs a Transactionparty
func NewTransactionParty(iban *iban.IBAN, name *string) TransactionParty {
	return TransactionParty{
		IBAN:    utils.EmptyIBANOrValue(iban),
		HasIBAN: iban != nil,
		Name:    utils.EmptyStringOrValue(name),
		HasName: name != nil,
	}
}

type recurringTransactionInstance struct {
	ID              primitives.RecurringTransactionInstanceID
	amount          money.Money
	from            iban.IBAN
	to              *iban.IBAN
	transactionDate time.Time
}

type recurringTransactionDetails struct {
	initialized         bool
	source              primitives.Source
	startDate           time.Time
	endDate             *time.Time
	frequency           period.Period
	amount              money.Money
	lastTransactionDate *time.Time
}

type recurringTransactionState struct {
	ID                            primitives.RecurringTransactionID
	details                       recurringTransactionDetails
	recurringTransactionInstances map[primitives.RecurringTransactionInstanceID]recurringTransactionInstance
	status                        Status
}

type RecurringTransactionEvent interface {
	appliedTo(state *recurringTransactionState) *recurringTransactionState
}

type RecurringTransactionCommand interface {
	applyTo(state *recurringTransactionState) ([]RecurringTransactionEvent, []scheduledRecurringTransactionCommand)
}

type scheduledRecurringTransactionCommand struct {
	RecurringTransactionCommand
	Identifier string
	When       time.Time
}

type ProcessScheduleCommand struct {
	RecurringTransactionID primitives.RecurringTransactionID
	Institution            primitives.Institution
	InstitutionEntityID    string
	FromIBAN               iban.IBAN
	ToIBAN                 iban.IBAN
	ToName                 string
	Frequency              period.Period
	StartDate              time.Time
	EndDate                *time.Time
	Amount                 primitives.MoneyForCommand
	FetchTimestamp         time.Time
}

func (cmd *ProcessScheduleCommand) endsAfter(t time.Time) bool {
	return cmd.EndDate != nil && cmd.EndDate.Before(t)
}

func (cmd ProcessScheduleCommand) applyTo(state *recurringTransactionState) ([]RecurringTransactionEvent, []scheduledRecurringTransactionCommand) {
	if !state.details.initialized {
		return []RecurringTransactionEvent{
			newNewRecurringTransactionFoundFromSchedule(cmd),
		}, nil
	}

	if state.details.source != primitives.Schedule {
		return []RecurringTransactionEvent{}, nil
	}

	var events []RecurringTransactionEvent

	if cmd.StartDate != state.details.startDate {
		events = append(events, newRecurringTransactionStartDateChanged(state.ID, cmd.StartDate))
	}

	if cmd.endsAfter(time.Now()) && state.status != Ended {
		events = append(events, newRecurringTransactionEnded(state.ID))
	} else if !cmd.endsAfter(time.Now()) && state.status == Ended {
		events = append(events, newRecurringTransactionReopened(state.ID))
	}

	if cmd.Amount.ToMoney() != state.details.amount {
		events = append(events, newRecurringTransactionAmountChanged(state.ID, cmd.Amount.ToMoney()))
	}

	if cmd.Frequency != state.details.frequency {
		events = append(events, newRecurringTransactionFrequencyChanged(state.ID, cmd.Frequency))
	}

	return events, nil
}

type ProcessDirectDebitTransactionDocumentCommand struct {
	RecurringTransactionID primitives.RecurringTransactionID
	TransactionID          primitives.RecurringTransactionInstanceID
	Institution            primitives.Institution
	InstititionEntityID    string
	From                   TransactionParty
	To                     TransactionParty
	TransactionDate        time.Time
	Amount                 primitives.MoneyForCommand
	FetchTimestamp         time.Time
}

func (cmd ProcessDirectDebitTransactionDocumentCommand) applyTo(state *recurringTransactionState) ([]RecurringTransactionEvent, []scheduledRecurringTransactionCommand) {
	if !state.details.initialized {
		return []RecurringTransactionEvent{
			newNewRecurringTransactionFoundFromDirectDebit(cmd, period.NewYMD(0, 1, 0)),
		}, nil
	}

	if state.details.source != primitives.DirectDebit {
		return []RecurringTransactionEvent{}, nil
	}

	var events []RecurringTransactionEvent
	var scheduledCommands []scheduledRecurringTransactionCommand

	if cmd.TransactionDate.Before(state.details.startDate) {
		events = append(events, newRecurringTransactionStartDateChanged(state.ID, cmd.TransactionDate))
	}

	_, hasTransaction := state.recurringTransactionInstances[cmd.TransactionID]
	if !hasTransaction {
		events = append(events, newNewRecurringTransactionInstanceFound(cmd.TransactionID, state.ID, cmd.Amount.ToMoney(), cmd.From, cmd.To, cmd.TransactionDate))
	}

	if cmd.Amount.ToMoney() != state.details.amount && (state.details.lastTransactionDate == nil || cmd.TransactionDate.After(*state.details.lastTransactionDate)) {
		events = append(events, newRecurringTransactionAmountChanged(state.ID, cmd.Amount.ToMoney()))
	}

	transactionDates := append(transactionDatesFrom(state.recurringTransactionInstances), cmd.TransactionDate)
	frequency := getFrequencyFor(transactionDates)
	if frequency != state.details.frequency {
		events = append(events, newRecurringTransactionFrequencyChanged(state.ID, frequency))
	}

	recheckCommmand := *newRecheckStatusCommand()
	scheduledCommands = append(scheduledCommands, scheduledRecurringTransactionCommand{recheckCommmand, "1-min", time.Now().Add(time.Second * time.Duration(60))})

	scheduledCommands = append(scheduledCommands, scheduledRecurringTransactionCommand{recheckCommmand, "after-frequency", endOfNextWorkDayAfter(time.Now().Add(frequency.DurationApprox()))})

	return events, scheduledCommands
}

type ProcessScheduledTransactionCommand struct {
	RecurringTransactionID primitives.RecurringTransactionID
	ID                     primitives.RecurringTransactionInstanceID // should this be a func?
	Amount                 money.Money
	From                   TransactionParty
	To                     TransactionParty
	TransactionDate        time.Time
	FetchTime              time.Time
}

func (cmd ProcessScheduledTransactionCommand) applyTo(state *recurringTransactionState) ([]RecurringTransactionEvent, []scheduledRecurringTransactionCommand) {
	if state.details.source != primitives.Schedule {
		return nil, nil
	}

	var events []RecurringTransactionEvent

	_, hasTransaction := state.recurringTransactionInstances[cmd.ID]
	if !hasTransaction {
		events = append(events, newNewRecurringTransactionInstanceFound(cmd.ID, state.ID, cmd.Amount, cmd.From, cmd.To, cmd.TransactionDate))
	}

	return events, nil
}

type RecheckStatusCommand struct {
	RecurringTransactionID primitives.RecurringTransactionID
}

func newRecheckStatusCommand() *RecheckStatusCommand {
	res := new(RecheckStatusCommand)
	return res
}

func (cmd RecheckStatusCommand) applyTo(state *recurringTransactionState) ([]RecurringTransactionEvent, []scheduledRecurringTransactionCommand) {
	if !state.details.initialized {
		return nil, nil
	}

	var events []RecurringTransactionEvent

	lastTransactionDate := state.details.lastTransactionDate
	frequency := state.details.frequency

	if state.status == Active {
		if state.details.endDate != nil && state.details.endDate.Before(time.Now()) {
			// ENDED
		} else if state.details.lastTransactionDate != nil && endOfNextWorkDayAfter(lastTransactionDate.Add(frequency.DurationApprox())).Before(time.Now()) {
			// ENDED
		}
	}

	if state.status == Ended {
		if state.details.lastTransactionDate != nil && endOfNextWorkDayAfter(lastTransactionDate.Add(frequency.DurationApprox())).After(time.Now()) {
			// REOPENED
		}
	}

	return events, nil
}

type NewRecurringTransactionFound struct {
	From      TransactionParty
	To        TransactionParty
	Frequency period.Period
	Amount    money.Money
	Source    primitives.Source
	StartDate time.Time
	EndDate   *time.Time
}

func newNewRecurringTransactionFoundFromSchedule(cmd ProcessScheduleCommand) NewRecurringTransactionFound {
	res := new(NewRecurringTransactionFound)
	res.Amount = cmd.Amount.ToMoney()
	res.EndDate = cmd.EndDate
	res.StartDate = cmd.StartDate
	// TODO
	// res.From = cmd.From
	// res.To = cmd.To
	res.Frequency = cmd.Frequency
	res.Source = primitives.Schedule
	return *res
}

func newNewRecurringTransactionFoundFromDirectDebit(cmd ProcessDirectDebitTransactionDocumentCommand, frequency period.Period) NewRecurringTransactionFound {
	res := new(NewRecurringTransactionFound)
	res.Amount = cmd.Amount.ToMoney()
	res.StartDate = cmd.TransactionDate
	// TODO
	// res.From = cmd.FromIBAN
	// res.To = cmd.ToIBAN
	res.Frequency = frequency
	res.Source = primitives.DirectDebit
	return *res
}

func (event NewRecurringTransactionFound) appliedTo(state *recurringTransactionState) *recurringTransactionState {
	res := recurringTransactionState{}
	copier.Copy(&res, &state)

	res.status = Active
	res.details.initialized = true
	res.details.amount = event.Amount
	res.details.startDate = event.StartDate
	res.details.endDate = event.EndDate
	res.details.frequency = event.Frequency
	res.details.source = event.Source

	return &res
}

type RecurringTransactionAmountChanged struct {
	ID     primitives.RecurringTransactionID
	Amount money.Money
}

func newRecurringTransactionAmountChanged(id primitives.RecurringTransactionID, amount money.Money) RecurringTransactionAmountChanged {
	res := new(RecurringTransactionAmountChanged)
	res.ID = id
	res.Amount = amount
	return *res
}

func (event RecurringTransactionAmountChanged) appliedTo(state *recurringTransactionState) *recurringTransactionState {
	res := recurringTransactionState{}
	copier.Copy(&res, &state)

	res.details.amount = event.Amount
	return &res
}

type RecurringTransactionFrequencyChanged struct {
	ID        primitives.RecurringTransactionID
	Frequency period.Period
}

func newRecurringTransactionFrequencyChanged(id primitives.RecurringTransactionID, frequency period.Period) RecurringTransactionFrequencyChanged {
	res := new(RecurringTransactionFrequencyChanged)
	res.ID = id
	res.Frequency = frequency
	return *res
}

func (event RecurringTransactionFrequencyChanged) appliedTo(state *recurringTransactionState) *recurringTransactionState {
	res := recurringTransactionState{}
	copier.Copy(&res, &state)

	res.details.frequency = event.Frequency
	return &res
}

type RecurringTransactionEnded struct {
	ID primitives.RecurringTransactionID
}

func newRecurringTransactionEnded(id primitives.RecurringTransactionID) RecurringTransactionEnded {
	res := new(RecurringTransactionEnded)
	res.ID = id
	return *res
}

func (event RecurringTransactionEnded) appliedTo(state *recurringTransactionState) *recurringTransactionState {
	res := recurringTransactionState{}
	copier.Copy(&res, &state)

	res.status = Ended
	return &res
}

type RecurringTransactionStartDateChanged struct {
	ID        primitives.RecurringTransactionID
	StartDate time.Time
}

func newRecurringTransactionStartDateChanged(id primitives.RecurringTransactionID, startDate time.Time) RecurringTransactionStartDateChanged {
	res := new(RecurringTransactionStartDateChanged)
	res.ID = id
	res.StartDate = startDate
	return *res
}

func (event RecurringTransactionStartDateChanged) appliedTo(state *recurringTransactionState) *recurringTransactionState {
	res := recurringTransactionState{}
	copier.Copy(&res, &state)

	res.details.startDate = event.StartDate
	return &res
}

type RecurringTransactionReopened struct {
	ID primitives.RecurringTransactionID
}

func newRecurringTransactionReopened(id primitives.RecurringTransactionID) RecurringTransactionReopened {
	res := new(RecurringTransactionReopened)
	res.ID = id
	return *res
}

func (event RecurringTransactionReopened) appliedTo(state *recurringTransactionState) *recurringTransactionState {
	res := recurringTransactionState{}
	copier.Copy(&res, &state)

	res.status = Active
	return &res
}

type NewRecurringTransactionInstanceFound struct {
	ID                   primitives.RecurringTransactionInstanceID
	RecurringTransaction primitives.RecurringTransactionID
	Amount               money.Money
	From                 iban.IBAN
	To                   *iban.IBAN
	TransactionDate      time.Time
}

func newNewRecurringTransactionInstanceFound(id primitives.RecurringTransactionInstanceID, recurringTransaction primitives.RecurringTransactionID, amount money.Money, from TransactionParty, to TransactionParty, transactionDate time.Time) NewRecurringTransactionInstanceFound {
	res := new(NewRecurringTransactionInstanceFound)
	res.ID = id
	res.RecurringTransaction = recurringTransaction
	res.Amount = amount
	// TODO
	// res.From = from
	// res.To = to
	res.TransactionDate = transactionDate
	return *res
}

func (event NewRecurringTransactionInstanceFound) appliedTo(state *recurringTransactionState) *recurringTransactionState {
	res := recurringTransactionState{}
	copier.Copy(&res, &state)

	res.recurringTransactionInstances[event.ID] = recurringTransactionInstance{
		ID:              event.ID,
		amount:          event.Amount,
		from:            event.From,
		to:              event.To,
		transactionDate: event.TransactionDate,
	}

	if res.details.lastTransactionDate == nil || event.TransactionDate.After(*res.details.lastTransactionDate) {
		res.details.lastTransactionDate = &event.TransactionDate
	}

	return &res
}

func getFrequencyFor(transactionDates []time.Time) period.Period {
	return period.NewYMD(0, 1, 0) // TODO
}

func transactionDatesFrom(vs map[primitives.RecurringTransactionInstanceID]recurringTransactionInstance) []time.Time {
	res := make([]time.Time, 0, len(vs))
	for _, v := range vs {
		res = append(res, v.transactionDate)
	}
	return res
}

func endOfNextWorkDayAfter(after time.Time) time.Time {
	c := cal.NewBusinessCalendar()

	for _, holiday := range nl.Holidays {
		c.AddHoliday(holiday)
	}

	startOfNextBusinessDay := c.NextWorkdayStart(after)
	endOfNextBusinessDay := now.With(startOfNextBusinessDay).EndOfDay()

	return endOfNextBusinessDay
}
