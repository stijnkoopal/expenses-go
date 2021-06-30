package recurring

import (
	"app/primitives"
	"testing"

	"github.com/google/uuid"
)

var recurringTransactionId = primitives.RecurringTransactionID(uuid.New())

func Test_NewRecurringTransactionFound_InitializesTransaction(t *testing.T) {
	// state := EmptyMonetaryAccountState(monetaryAccountID)
	// laterTimestampEvent := MonetaryAccountBalanceSnapshotted{balance: *money.New(0, "EUR"), timestamp: time.Now().AddDate(0, 1, 0)}
	// formerTimestampEvent := MonetaryAccountBalanceSnapshotted{balance: *money.New(0, "EUR"), timestamp: time.Now().AddDate(0, -1, 0)}

	// result := formerTimestampEvent.appliedTo(laterTimestampEvent.appliedTo(state))

	// if result.BalanceHistory[0].timestamp.After(result.BalanceHistory[1].timestamp) {
	// 	t.Errorf("Balances not sorted on timestamp")
	// }
}

func Test_RecurringTransactionAmountChanged_ChangesAmount(t *testing.T) {
}

func Test_RecurringTransactionFrequencyChanged_ChangesFrequency(t *testing.T) {
}

func Test_RecurringTransactionEnded_Ends(t *testing.T) {
}

func Test_RecurringTransactionReopened_Reopens(t *testing.T) {
}

func Test_RecurringTransactionStartDateChanged_ChangesStartDate(t *testing.T) {
}

func Test_NewRecurringTransactionInstanceFound_AddsNewTransaction(t *testing.T) {
}

func Test_NewRecurringTransactionInstanceFound_NoopOnExistingTransaction(t *testing.T) {
}

func Test_NewRecurringTransactionInstanceFound_SetsLastTransactionDate(t *testing.T) {
}
