package accountinformation

import (
	"app/utils"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/aggregatestore/events"
	"github.com/looplab/eventhorizon/commandhandler/aggregate"
)

func SetupDomain(
	eventStore eh.EventStore,
	eventBus eh.EventBus,
) (eh.CommandHandler, error) {
	aggregateStore, err := events.NewAggregateStore(eventStore, eventBus)
	if err != nil {
		return nil, fmt.Errorf("could not create aggregate store: %w", err)
	}

	commandHandler, err := aggregate.NewCommandHandler(MonetaryAccountAggregateType, aggregateStore)
	if err != nil {
		return nil, fmt.Errorf("could not create command handler: %w", err)
	}

	return commandHandler, nil
}

// AggregateType is the aggregate type for the monetary account
const MonetaryAccountAggregateType = eh.AggregateType("monetaryaccount")

// Aggregate is an aggregate for a monetary account
type Aggregate struct {
	*events.AggregateBase
	*MonetaryAccountState
}

const EhProcessMonetaryAccountCommand = eh.CommandType("monetaryaccount:proces")
const EhProcessTransactionDocumentCommand = eh.CommandType("monetaryaccount:proces-tx")
const EhUpdateBalanceForNonAutomatedAccountCommand = eh.CommandType("monetaryaccount:update-balance-non-automated")

const EhNewMonetaryAccountFound = eh.EventType("monetaryaccount:new-found")
const EhMonetaryAccountBecameJoint = eh.EventType("monetaryaccount:became-joint")
const EhMonetaryAccountBecameSingular = eh.EventType("monetaryaccount:became-singular")
const EhMonetaryAccountAliasUpdated = eh.EventType("monetaryaccount:alias-updated")
const EhNewTransactionFound = eh.EventType("monetaryaccount:new-tx")
const EhMonetaryAccountBalanceSnapshotted = eh.EventType("monetaryaccount:balance-snapshotted")
const EhMonetaryAccountUserAdded = eh.EventType("monetaryaccount:user-added")

func (cmd ProcessMonetaryAccountCommand) AggregateID() uuid.UUID {
	return uuid.UUID(cmd.MonetaryAccountID)
}

func (cmd ProcessMonetaryAccountCommand) AggregateType() eh.AggregateType {
	return MonetaryAccountAggregateType
}

func (cmd ProcessMonetaryAccountCommand) CommandType() eh.CommandType {
	return EhProcessMonetaryAccountCommand
}

func (cmd ProcessTransactionDocumentCommand) AggregateID() uuid.UUID {
	return uuid.UUID(cmd.MonetaryAccountID)
}

func (cmd ProcessTransactionDocumentCommand) AggregateType() eh.AggregateType {
	return MonetaryAccountAggregateType
}

func (cmd ProcessTransactionDocumentCommand) CommandType() eh.CommandType {
	return EhProcessTransactionDocumentCommand
}

func (cmd UpdateBalanceForNonAutomatedAccountCommand) AggregateID() uuid.UUID {
	return uuid.UUID(cmd.MonetaryAccountID)
}

func (cmd UpdateBalanceForNonAutomatedAccountCommand) AggregateType() eh.AggregateType {
	return MonetaryAccountAggregateType
}

func (cmd UpdateBalanceForNonAutomatedAccountCommand) CommandType() eh.CommandType {
	return EhUpdateBalanceForNonAutomatedAccountCommand
}

func init() {
	eh.RegisterAggregate(func(id uuid.UUID) eh.Aggregate {
		return &Aggregate{
			AggregateBase: events.NewAggregateBase(MonetaryAccountAggregateType, id),
		}
	})

	eh.RegisterEventData(EhNewMonetaryAccountFound, func() eh.EventData {
		return &NewMonetaryAccountFound{}
	})

	eh.RegisterEventData(EhMonetaryAccountBecameJoint, func() eh.EventData {
		return &MonetaryAccountBecameJoint{}
	})

	eh.RegisterEventData(EhMonetaryAccountBecameSingular, func() eh.EventData {
		return &MonetaryAccountBecameSingular{}
	})

	eh.RegisterEventData(EhMonetaryAccountAliasUpdated, func() eh.EventData {
		return &MonetaryAccountAliasUpdated{}
	})

	eh.RegisterEventData(EhNewTransactionFound, func() eh.EventData {
		return &NewTransactionFound{}
	})

	eh.RegisterEventData(EhMonetaryAccountUserAdded, func() eh.EventData {
		return &MonetaryAccountUserAdded{}
	})

	eh.RegisterEventData(EhMonetaryAccountBalanceSnapshotted, func() eh.EventData {
		return &MonetaryAccountBalanceSnapshotted{}
	})
}

// HandleCommand implements the HandleCommand method of the eventhorizon.CommandHandler interface.
func (a *Aggregate) HandleCommand(ctx context.Context, cmd eh.Command) error {
	domainCommand, err := mapToDomainCommand(cmd)
	if err != nil {
		return err
	}

	events := domainCommand.applyTo(a.MonetaryAccountState)
	for _, event := range events {
		eventType, err := mapToEhEventType(event)
		if err != nil {
			log.Printf("Could not map event, %s", err)
		} else {
			a.AppendEvent(eventType, event, time.Now())
		}
	}

	return nil
}

// ApplyEvent implements the ApplyEvent method of the eventhorizon.Aggregate interface.
func (a *Aggregate) ApplyEvent(ctx context.Context, event eh.Event) error {
	eventInDomain, err := mapToDomainEvent(event)
	if err != nil {
		return fmt.Errorf("unable to understand evnt %v", event)
	}
	a.MonetaryAccountState = eventInDomain.appliedTo(a.MonetaryAccountState)
	return nil
}

func mapToDomainEvent(event eh.Event) (MonetaryAccountEvent, error) {
	switch event.EventType() {
	case EhNewMonetaryAccountFound:
		return event.Data().(MonetaryAccountEvent), nil
	case EhMonetaryAccountBecameSingular:
		return event.Data().(MonetaryAccountEvent), nil
	case EhMonetaryAccountBecameJoint:
		return event.Data().(MonetaryAccountEvent), nil
	case EhMonetaryAccountAliasUpdated:
		return event.Data().(MonetaryAccountEvent), nil
	case EhNewTransactionFound:
		return event.Data().(MonetaryAccountEvent), nil
	case EhMonetaryAccountUserAdded:
		return event.Data().(MonetaryAccountEvent), nil
	case EhMonetaryAccountBalanceSnapshotted:
		return event.Data().(MonetaryAccountEvent), nil
	default:
		return nil, fmt.Errorf("unable to understand evnt %v", event)
	}
}

func mapToEhEventType(event MonetaryAccountEvent) (eh.EventType, error) {
	switch event.(type) {
	case NewMonetaryAccountFound:
		return EhNewMonetaryAccountFound, nil
	case MonetaryAccountBecameJoint:
		return EhMonetaryAccountBecameJoint, nil
	case MonetaryAccountBecameSingular:
		return EhMonetaryAccountBecameSingular, nil
	case MonetaryAccountAliasUpdated:
		return EhMonetaryAccountAliasUpdated, nil
	case NewTransactionFound:
		return EhNewTransactionFound, nil
	case MonetaryAccountUserAdded:
		return EhMonetaryAccountUserAdded, nil
	case MonetaryAccountBalanceSnapshotted:
		return EhMonetaryAccountBalanceSnapshotted, nil
	}
	return "", fmt.Errorf("Could not understand event of type %s", utils.TypeNameOf(event))
}

func mapToDomainCommand(cmd eh.Command) (MonetaryAccountCommand, error) {
	switch cmd := cmd.(type) {
	case ProcessMonetaryAccountCommand:
		return cmd, nil
	case ProcessTransactionDocumentCommand:
		return cmd, nil
	case UpdateBalanceForNonAutomatedAccountCommand:
		return cmd, nil

	default:
		return nil, fmt.Errorf("Could not understand command of type %s", utils.TypeNameOf(cmd))
	}
}
