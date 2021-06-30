package accountinformation

import (
	"app/primitives"
	"app/utils"
	"sort"
	"time"

	"github.com/Rhymond/go-money"
	"github.com/almerlucke/go-iban/iban"
	"github.com/jinzhu/copier"
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

// Transaction base transaction
type Transaction struct {
	from            TransactionParty
	to              TransactionParty
	amount          money.Money
	transactionTime time.Time
}

// NewTransaction constructs a Transaction
func NewTransaction(from TransactionParty, to TransactionParty, amount money.Money, transactionTime time.Time) Transaction {
	t := new(Transaction)
	t.from = from
	t.to = to
	t.amount = amount
	t.transactionTime = transactionTime
	return *t
}

type monetaryAccountDetails struct {
	initialized bool
	iban        iban.IBAN
	joint       bool
	institution primitives.Institution
	alias       string
	currency    money.Currency
}

type balanceHistory struct {
	balance   money.Money
	timestamp time.Time
}

type MonetaryAccountState struct {
	ID             primitives.MonetaryAccountID
	Details        monetaryAccountDetails
	Transactions   map[primitives.TransactionID]Transaction
	BalanceHistory []balanceHistory
	Owners         map[primitives.UserID]primitives.UserID
}

func EmptyMonetaryAccountState(ID primitives.MonetaryAccountID) *MonetaryAccountState {
	res := new(MonetaryAccountState)
	res.ID = ID
	res.Details = monetaryAccountDetails{}
	res.Transactions = make(map[primitives.TransactionID]Transaction, 0)
	res.BalanceHistory = make([]balanceHistory, 0)
	res.Owners = make(map[primitives.UserID]primitives.UserID)
	return res
}

type MonetaryAccountEvent interface {
	appliedTo(state *MonetaryAccountState) *MonetaryAccountState
}

type MonetaryAccountCommand interface {
	applyTo(state *MonetaryAccountState) []MonetaryAccountEvent
}

type ProcessMonetaryAccountCommand struct {
	MonetaryAccountID   primitives.MonetaryAccountID
	Iban                iban.IBAN
	Joint               bool
	OwnerUserID         primitives.UserID
	Alias               string
	Institution         primitives.Institution
	InstitutionEntityID string
	Balance             primitives.MoneyForCommand
	FetchTimestamp      time.Time
}

func (cmd ProcessMonetaryAccountCommand) applyTo(state *MonetaryAccountState) []MonetaryAccountEvent {
	if state == nil || !state.Details.initialized {
		return []MonetaryAccountEvent{
			newNewMonetaryAccountFound(cmd),
			newBalanceHistorySnapshotted(cmd),
			newMonetaryAccountUserAdded(cmd),
		}
	}

	var events []MonetaryAccountEvent
	if state.Details.alias != cmd.Alias {
		events = append(events, newMonetaryAccountAliasUpdated(cmd))
	}

	if state.Details.joint && !cmd.Joint {
		events = append(events, newMonetaryAccountBecameSingular(cmd))
	} else if !state.Details.joint && cmd.Joint {
		events = append(events, newMonetaryAccountBecameJoint(cmd))
	}

	lastBalance := state.BalanceHistory[len(state.BalanceHistory)-1]
	if lastBalance.balance.Amount() != cmd.Balance.Amount || lastBalance.timestamp.Add(time.Hour*1).Before(cmd.FetchTimestamp) {
		events = append(events, newBalanceHistorySnapshotted(cmd))
	}

	_, hasOwner := state.Owners[cmd.OwnerUserID]

	if !hasOwner {
		events = append(events, newMonetaryAccountUserAdded(cmd))
	}

	return events
}

type ProcessTransactionDocumentCommand struct {
	ID primitives.TransactionID

	MonetaryAccountID primitives.MonetaryAccountID

	FromMonetaryAccountID primitives.MonetaryAccountID
	From                  TransactionParty

	ToMonetaryAccountID primitives.MonetaryAccountID
	To                  TransactionParty

	InstitutionEntityID   string
	Amount                primitives.MoneyForCommand
	Description           string `eh:"optional"`
	InstitutionScheduleID string `eh:"optional"`
	IsScheduled           bool
	BalanceAfterMutation  primitives.MoneyForCommand
	TransactionDate       time.Time
	FetchTimestamp        time.Time
}

func (cmd ProcessTransactionDocumentCommand) applyTo(state *MonetaryAccountState) []MonetaryAccountEvent {
	if state == nil {
		return nil
	}

	var events []MonetaryAccountEvent

	_, hasTransaction := state.Transactions[cmd.ID]
	if !hasTransaction {
		events = append(events, newNewTransactionFound(cmd))
	}
	return events
}

type UpdateBalanceForNonAutomatedAccountCommand struct {
	MonetaryAccountID primitives.MonetaryAccountID
}

func (cmd UpdateBalanceForNonAutomatedAccountCommand) applyTo(state *MonetaryAccountState) []MonetaryAccountEvent {
	// TODO
	return nil
}

type NewTransactionFound struct {
	ID primitives.TransactionID
}

func newNewTransactionFound(cmd ProcessTransactionDocumentCommand) NewTransactionFound {
	res := new(NewTransactionFound)
	// TODO
	return *res
}

func (event NewTransactionFound) appliedTo(state *MonetaryAccountState) *MonetaryAccountState {
	_, hasTransaction := state.Transactions[event.ID]
	if hasTransaction {
		return state
	}

	res := MonetaryAccountState{}
	copier.Copy(&res, &state)

	// TODO
	res.Transactions[event.ID] = Transaction{}

	return &res
}

type NewMonetaryAccountFound struct {
	ID           primitives.MonetaryAccountID
	Iban         iban.IBAN
	Joint        bool
	OwnerUserIds []primitives.UserID
	Alias        string
	Institution  primitives.Institution
	Currency     money.Currency
}

func newNewMonetaryAccountFound(cmd ProcessMonetaryAccountCommand) NewMonetaryAccountFound {
	res := new(NewMonetaryAccountFound)
	res.ID = cmd.MonetaryAccountID
	res.Iban = cmd.Iban
	res.Joint = cmd.Joint
	res.OwnerUserIds = []primitives.UserID{cmd.OwnerUserID}
	res.Alias = cmd.Alias
	res.Institution = cmd.Institution
	res.Currency = *money.GetCurrency(cmd.Balance.CurrencyCode)
	return *res
}

func (event NewMonetaryAccountFound) appliedTo(state *MonetaryAccountState) *MonetaryAccountState {
	res := EmptyMonetaryAccountState(event.ID)

	res.Details.initialized = true
	res.Details.alias = event.Alias
	res.Details.currency = event.Currency
	res.Details.iban = event.Iban
	res.Details.institution = event.Institution
	res.Details.joint = event.Joint
	return res
}

type MonetaryAccountBecameJoint struct {
	ID primitives.MonetaryAccountID
}

func newMonetaryAccountBecameJoint(cmd ProcessMonetaryAccountCommand) MonetaryAccountBecameJoint {
	res := new(MonetaryAccountBecameJoint)
	res.ID = cmd.MonetaryAccountID
	return *res
}

func (event MonetaryAccountBecameJoint) appliedTo(state *MonetaryAccountState) *MonetaryAccountState {
	res := MonetaryAccountState{}
	copier.Copy(&res, &state)

	res.Details.joint = true
	return &res
}

type MonetaryAccountBecameSingular struct {
	ID primitives.MonetaryAccountID
}

func newMonetaryAccountBecameSingular(cmd ProcessMonetaryAccountCommand) MonetaryAccountBecameSingular {
	res := new(MonetaryAccountBecameSingular)
	res.ID = cmd.MonetaryAccountID
	return *res
}

func (event MonetaryAccountBecameSingular) appliedTo(state *MonetaryAccountState) *MonetaryAccountState {
	res := MonetaryAccountState{}
	copier.Copy(&res, &state)

	res.Details.joint = false
	return &res
}

type MonetaryAccountAliasUpdated struct {
	ID    primitives.MonetaryAccountID
	Alias string
}

func newMonetaryAccountAliasUpdated(cmd ProcessMonetaryAccountCommand) MonetaryAccountAliasUpdated {
	res := new(MonetaryAccountAliasUpdated)
	res.ID = cmd.MonetaryAccountID
	res.Alias = cmd.Alias
	return *res
}

func (event MonetaryAccountAliasUpdated) appliedTo(state *MonetaryAccountState) *MonetaryAccountState {
	res := MonetaryAccountState{}
	copier.Copy(&res, &state)

	res.Details.alias = event.Alias
	return &res
}

type MonetaryAccountUserAdded struct {
	ID     primitives.MonetaryAccountID
	UserID primitives.UserID
}

func newMonetaryAccountUserAdded(cmd ProcessMonetaryAccountCommand) MonetaryAccountUserAdded {
	res := new(MonetaryAccountUserAdded)
	res.ID = cmd.MonetaryAccountID
	res.UserID = cmd.OwnerUserID
	return *res
}

func (event MonetaryAccountUserAdded) appliedTo(state *MonetaryAccountState) *MonetaryAccountState {
	res := MonetaryAccountState{}
	copier.Copy(&res, &state)

	res.Owners[event.UserID] = event.UserID
	return &res
}

type MonetaryAccountBalanceSnapshotted struct {
	ID        primitives.MonetaryAccountID
	Balance   money.Money
	Timestamp time.Time
}

func newBalanceHistorySnapshotted(cmd ProcessMonetaryAccountCommand) MonetaryAccountBalanceSnapshotted {
	res := new(MonetaryAccountBalanceSnapshotted)
	res.ID = cmd.MonetaryAccountID
	res.Balance = *money.New(cmd.Balance.Amount, cmd.Balance.CurrencyCode)
	res.Timestamp = cmd.FetchTimestamp
	return *res
}

func (event MonetaryAccountBalanceSnapshotted) appliedTo(state *MonetaryAccountState) *MonetaryAccountState {
	res := MonetaryAccountState{}
	copier.Copy(&res, &state)

	res.BalanceHistory = append(res.BalanceHistory, balanceHistory{balance: event.Balance, timestamp: event.Timestamp})
	sort.Slice(res.BalanceHistory, func(i, j int) bool {
		return res.BalanceHistory[i].timestamp.Before(res.BalanceHistory[j].timestamp)
	})
	return &res
}
