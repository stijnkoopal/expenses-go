package recurring

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

	commandHandler, err := aggregate.NewCommandHandler(RecurringTransactionAggregateType, aggregateStore)
	if err != nil {
		return nil, fmt.Errorf("could not create command handler: %w", err)
	}

	return commandHandler, nil
}

// RecurringTransactionAggregateType is the aggregate type for the recurring transaction
const RecurringTransactionAggregateType = eh.AggregateType("recurringTransaction")

// Aggregate is an aggregate for a monetary account
type Aggregate struct {
	*events.AggregateBase
	*recurringTransactionState
}

const EhProcessScheduleCommand = eh.CommandType("recurring:process-schedule")
const EhProcessDirectDebitTransactionDocumentCommand = eh.CommandType("recurring:process-direct-debit-tx")
const EhProcessScheduledTransactionCommand = eh.CommandType("recurring:process-scheduled-tx")
const EhRecheckStatusCommand = eh.CommandType("recurring:recheck")

const EhNewRecurringTransactionFound = eh.EventType("recurring:new-found")
const EhRecurringTransactionAmountChanged = eh.EventType("recurring:amount-changed")
const EhRecurringTransactionFrequencyChanged = eh.EventType("recurring:frequency-changed")
const EhRecurringTransactionEnded = eh.EventType("recurring:ended")
const EhRecurringTransactionStartDateChanged = eh.EventType("recurring:start-date-changed")
const EhRecurringTransactionReopened = eh.EventType("recurring:reopened")
const EhNewRecurringTransactionInstanceFound = eh.EventType("recurring:instance-found")

func (cmd ProcessScheduleCommand) AggregateID() uuid.UUID {
	return uuid.UUID(cmd.RecurringTransactionID)
}

func (cmd ProcessScheduleCommand) AggregateType() eh.AggregateType {
	return RecurringTransactionAggregateType
}

func (cmd ProcessScheduleCommand) CommandType() eh.CommandType {
	return EhProcessScheduleCommand
}

func (cmd ProcessDirectDebitTransactionDocumentCommand) AggregateID() uuid.UUID {
	return uuid.UUID(cmd.RecurringTransactionID)
}

func (cmd ProcessDirectDebitTransactionDocumentCommand) AggregateType() eh.AggregateType {
	return RecurringTransactionAggregateType
}

func (cmd ProcessDirectDebitTransactionDocumentCommand) CommandType() eh.CommandType {
	return EhProcessDirectDebitTransactionDocumentCommand
}

func (cmd ProcessScheduledTransactionCommand) AggregateID() uuid.UUID {
	return uuid.UUID(cmd.RecurringTransactionID)
}

func (cmd ProcessScheduledTransactionCommand) AggregateType() eh.AggregateType {
	return RecurringTransactionAggregateType
}

func (cmd ProcessScheduledTransactionCommand) CommandType() eh.CommandType {
	return EhProcessScheduledTransactionCommand
}

func (cmd RecheckStatusCommand) AggregateID() uuid.UUID {
	return uuid.UUID(cmd.RecurringTransactionID)
}

func (cmd RecheckStatusCommand) AggregateType() eh.AggregateType {
	return RecurringTransactionAggregateType
}

func (cmd RecheckStatusCommand) CommandType() eh.CommandType {
	return EhRecheckStatusCommand
}

func init() {
	eh.RegisterAggregate(func(id uuid.UUID) eh.Aggregate {
		return &Aggregate{
			AggregateBase: events.NewAggregateBase(RecurringTransactionAggregateType, id),
		}
	})

	eh.RegisterEventData(EhNewRecurringTransactionFound, func() eh.EventData {
		return &NewRecurringTransactionFound{}
	})

	eh.RegisterEventData(EhRecurringTransactionAmountChanged, func() eh.EventData {
		return &RecurringTransactionAmountChanged{}
	})

	eh.RegisterEventData(EhRecurringTransactionFrequencyChanged, func() eh.EventData {
		return &RecurringTransactionFrequencyChanged{}
	})

	eh.RegisterEventData(EhRecurringTransactionEnded, func() eh.EventData {
		return &RecurringTransactionEnded{}
	})

	eh.RegisterEventData(EhRecurringTransactionStartDateChanged, func() eh.EventData {
		return &RecurringTransactionStartDateChanged{}
	})

	eh.RegisterEventData(EhRecurringTransactionReopened, func() eh.EventData {
		return &RecurringTransactionReopened{}
	})

	eh.RegisterEventData(EhNewRecurringTransactionInstanceFound, func() eh.EventData {
		return &NewRecurringTransactionInstanceFound{}
	})
}

// HandleCommand implements the HandleCommand method of the eventhorizon.CommandHandler interface.
func (a *Aggregate) HandleCommand(ctx context.Context, cmd eh.Command) error {
	domainCommand, err := mapToDomainCommand(cmd)
	if err != nil {
		return err
	}

	events, _ := domainCommand.applyTo(a.recurringTransactionState) // TODO: scheduled commands
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
	a.recurringTransactionState = eventInDomain.appliedTo(a.recurringTransactionState)
	return nil
}

func mapToDomainEvent(event eh.Event) (RecurringTransactionEvent, error) {
	switch event.EventType() {
	case EhNewRecurringTransactionFound:
		return event.Data().(RecurringTransactionEvent), nil
	case EhRecurringTransactionAmountChanged:
		return event.Data().(RecurringTransactionEvent), nil
	case EhRecurringTransactionFrequencyChanged:
		return event.Data().(RecurringTransactionEvent), nil
	case EhRecurringTransactionEnded:
		return event.Data().(RecurringTransactionEvent), nil
	case EhRecurringTransactionStartDateChanged:
		return event.Data().(RecurringTransactionEvent), nil
	case EhRecurringTransactionReopened:
		return event.Data().(RecurringTransactionEvent), nil
	case EhNewRecurringTransactionInstanceFound:
		return event.Data().(RecurringTransactionEvent), nil
	default:
		return nil, fmt.Errorf("unable to understand evnt %v", event)
	}
}

func mapToEhEventType(event RecurringTransactionEvent) (eh.EventType, error) {
	switch event.(type) {
	case NewRecurringTransactionFound:
		return EhNewRecurringTransactionFound, nil
	case RecurringTransactionAmountChanged:
		return EhRecurringTransactionAmountChanged, nil
	case RecurringTransactionFrequencyChanged:
		return EhRecurringTransactionFrequencyChanged, nil
	case RecurringTransactionEnded:
		return EhRecurringTransactionEnded, nil
	case RecurringTransactionStartDateChanged:
		return EhRecurringTransactionStartDateChanged, nil
	case RecurringTransactionReopened:
		return EhRecurringTransactionReopened, nil
	case NewRecurringTransactionInstanceFound:
		return EhNewRecurringTransactionInstanceFound, nil
	}
	return "", fmt.Errorf("Could not understand event of type %s", utils.TypeNameOf(event))
}

func mapToDomainCommand(cmd eh.Command) (RecurringTransactionCommand, error) {
	switch cmd := cmd.(type) {
	case ProcessScheduleCommand:
		return cmd, nil
	case ProcessDirectDebitTransactionDocumentCommand:
		return cmd, nil
	case ProcessScheduledTransactionCommand:
		return cmd, nil
	case RecheckStatusCommand:
		return cmd, nil

	default:
		return nil, fmt.Errorf("Could not understand command of type %s", utils.TypeNameOf(cmd))
	}
}
