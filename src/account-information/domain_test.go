package accountinformation

import (
	"app/primitives"
	"app/utils"
	"testing"
	"time"

	"github.com/Rhymond/go-money"

	"github.com/google/uuid"
)

var monetaryAccountID = primitives.MonetaryAccountID(uuid.New())

func Test_MonetaryAccountBalanceSnapshotted_SortedByTime(t *testing.T) {
	state := EmptyMonetaryAccountState(monetaryAccountID)
	laterTimestampEvent := MonetaryAccountBalanceSnapshotted{Balance: *money.New(0, "EUR"), Timestamp: time.Now().AddDate(0, 1, 0)}
	formerTimestampEvent := MonetaryAccountBalanceSnapshotted{Balance: *money.New(0, "EUR"), Timestamp: time.Now().AddDate(0, -1, 0)}

	result := formerTimestampEvent.appliedTo(laterTimestampEvent.appliedTo(state))

	if result.BalanceHistory[0].timestamp.After(result.BalanceHistory[1].timestamp) {
		t.Errorf("Balances not sorted on timestamp")
	}
}

func Test_MonetaryAccountUserAdded_AddsUser(t *testing.T) {
	state := EmptyMonetaryAccountState(monetaryAccountID)
	event1 := MonetaryAccountUserAdded{UserID: primitives.UserID(uuid.New())}
	event2 := MonetaryAccountUserAdded{UserID: primitives.UserID(uuid.New())}

	result := event2.appliedTo(event1.appliedTo(state))

	if len(result.Owners) != 2 {
		t.Errorf("Owners could not be added")
	}
}

func Test_MonetaryAccountAliasUpdated_SetsAlias(t *testing.T) {
	state := EmptyMonetaryAccountState(monetaryAccountID)
	event1 := MonetaryAccountAliasUpdated{Alias: "1"}
	event2 := MonetaryAccountAliasUpdated{Alias: "2"}

	result := event2.appliedTo(event1.appliedTo(state))

	if result.Details.alias != "2" {
		t.Errorf("Alias not set")
	}
}

func Test_MonetaryAccountBecameJoint(t *testing.T) {
	state := EmptyMonetaryAccountState(monetaryAccountID)
	state.Details.joint = false
	event1 := MonetaryAccountBecameJoint{}

	result := event1.appliedTo(state)

	if result.Details.joint != true {
		t.Errorf("Monetary account did not became joint")
	}
}

func Test_MonetaryAccountBecameSingular(t *testing.T) {
	state := EmptyMonetaryAccountState(monetaryAccountID)
	state.Details.joint = true
	event1 := MonetaryAccountBecameSingular{}

	result := event1.appliedTo(state)

	if result.Details.joint != false {
		t.Errorf("Monetary account did not became singular")
	}
}

func newStateAfter(state *MonetaryAccountState, cmd MonetaryAccountCommand) *MonetaryAccountState {
	events := cmd.applyTo(state)
	for i := 0; i < len(events); i++ {
		state = events[i].appliedTo(state)
	}
	return state
}

func Test_ProcessMonetaryAccountCommand_Apply_IntialEvents(t *testing.T) {
	state := EmptyMonetaryAccountState(monetaryAccountID)
	cmd := ProcessMonetaryAccountCommand{
		Balance: primitives.NewMoneyForCommand(*money.New(0, "EUR")),
	}

	events := cmd.applyTo(state)

	expectedEventTypes := []string{"NewMonetaryAccountFound", "MonetaryAccountBalanceSnapshotted", "MonetaryAccountUserAdded"}
	if len(events) != len(expectedEventTypes) {
		t.Errorf("Expected %d events, found %d", len(expectedEventTypes), len(events))
	} else {
		for i := 0; i < len(expectedEventTypes); i++ {
			eventType := utils.TypeNameOf(events[i])
			if eventType != expectedEventTypes[i] {
				t.Errorf("Event %d is not %s, found %s", i, expectedEventTypes[i], eventType)
			}
		}
	}
}

func Test_ProcessMonetaryAccountCommand_Apply_NoChanges(t *testing.T) {
	state := EmptyMonetaryAccountState(monetaryAccountID)
	cmd := ProcessMonetaryAccountCommand{
		Balance: primitives.NewMoneyForCommand(*money.New(0, "EUR")),
	}

	state = newStateAfter(state, cmd)
	events := cmd.applyTo(state)

	if len(events) != 0 {
		t.Errorf("Expected zero event, found %d", len(events))
	}
}
