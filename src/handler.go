// Copyright (c) 2017 - The Event Horizon authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	accountinformation "app/account-information"
	"context"
	"fmt"
	"log"

	eh "github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/aggregatestore/events"
	eventbus "github.com/looplab/eventhorizon/eventbus/local"
	eventstore "github.com/looplab/eventhorizon/eventstore/memory"
	"github.com/looplab/eventhorizon/middleware/eventhandler/observer"
)

// Handler is a http.Handler for the TodoMVC app.
type Handler struct {
	EventBus       eh.EventBus
	CommandHandler eh.CommandHandler
	Repo           eh.ReadWriteRepo
}

func newEventStore() *eventstore.EventStore {
	return eventstore.NewEventStore()
}

func newEventBus() *eventbus.EventBus {
	result := eventbus.NewEventBus(nil)
	go func() {
		for e := range result.Errors() {
			log.Printf("eventbus: %s", e.Error())
		}
	}()

	return result
}

func newAggregateStore(store *eventstore.EventStore, bus *eventbus.EventBus) *events.AggregateStore {
	aggregateStore, err := events.NewAggregateStore(store, bus)
	if err != nil {
		panic(fmt.Errorf("could not create aggregate store: %s", err))
	}
	return aggregateStore
}

// NewHandler sets up the full Event Horizon domain for the TodoMVC app and
// returns a handler exposing some of the components.
func NewHandler() (*Handler, error) {
	eventStore := newEventStore()
	eventBus := newEventBus()

	eventBus.AddHandler(eh.MatchAny(),
		eh.UseEventHandlerMiddleware(&EventLogger{}, observer.Middleware))

	commandHandler, err := accountinformation.SetupDomain(eventStore, eventBus)
	if err != nil {
		return nil, err
	}

	// Create a tiny logging middleware for the command handler.
	commandHandlerLogger := func(h eh.CommandHandler) eh.CommandHandler {
		return eh.CommandHandlerFunc(func(ctx context.Context, cmd eh.Command) error {
			log.Printf("CMD %#v", cmd)
			return h.HandleCommand(ctx, cmd)
		})
	}
	commandHandler = eh.UseCommandHandlerMiddleware(commandHandler, commandHandlerLogger)

	// // Create the repository and wrap in a version repository.
	// repo := repo.NewRepo()
	// repo.SetEntityFactory(func() eh.Entity { return &domain.TodoList{} })
	// todoRepo := version.NewRepo(repo)

	// // Create the read model projector.
	// projector := projector.NewEventHandler(&domain.Projector{}, todoRepo)
	// projector.SetEntityFactory(func() eh.Entity { return &domain.TodoList{} })
	// eventBus.AddHandler(eh.MatchAnyEventOf(
	// 	domain.Created,
	// 	domain.Deleted,
	// 	domain.ItemAdded,
	// 	domain.ItemRemoved,
	// 	domain.ItemDescriptionSet,
	// 	domain.ItemChecked,
	// ), projector)

	return &Handler{
		EventBus:       eventBus,
		CommandHandler: commandHandler,
		// Repo:           todoRepo,
	}, nil
}

// EventLogger is a simple event handler for logging all events.
type EventLogger struct{}

// HandlerType implements the HandlerType method of the eventhorizon.EventHandler interface.
func (l *EventLogger) HandlerType() eh.EventHandlerType {
	return "logger"
}

// HandleEvent implements the HandleEvent method of the EventHandler interface.
func (l *EventLogger) HandleEvent(ctx context.Context, event eh.Event) error {
	log.Printf("EVENT %s", event)
	return nil
}
