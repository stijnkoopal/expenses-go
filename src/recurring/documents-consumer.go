package recurring

import (
	"app/bus"
	"app/primitives"
	"app/utils"
	"context"
	"log"
	"reflect"
	"sync"

	eh "github.com/looplab/eventhorizon"
)

type DocumentsConsumer interface {
	Start()
}

type DocumentsFromBusConsumer struct {
	context                               context.Context
	handler                               eh.CommandHandler
	recurringTransactionIDFetcher         RecurringTransactionIDFetcher
	recurringTransactionInstanceIDFetcher RecurringTransactionInstanceIDFetcher
}

func NewDocumentsFromBusConsumer(
	ctx context.Context,
	handler eh.CommandHandler,
) DocumentsConsumer {
	return DocumentsFromBusConsumer{context: ctx, handler: handler}
}

func (consumer DocumentsFromBusConsumer) Start() {
	wg := sync.WaitGroup{}

	directDebitChannel := bus.DirectDebitChannelForReading()
	scheduleChannel := bus.ScheduleChannelForReading()

	for {
		select {
		case document, ok := <-directDebitChannel:
			if !ok {
				directDebitChannel = nil
			} else {
				wg.Add(1)
				go consumer.handleDirectDebitDocument(&wg, document)
			}

		case document, ok := <-scheduleChannel:
			if !ok {
				scheduleChannel = nil
			} else {
				wg.Add(1)
				go consumer.handleScheduleDocument(&wg, document)
			}
		}

		if directDebitChannel == nil && scheduleChannel == nil {
			break
		}
	}

	wg.Wait()
}

func (consumer DocumentsFromBusConsumer) handleDirectDebitDocument(wg *sync.WaitGroup, document bus.DirectDebitTransactionDocument) {
	defer wg.Done()

	transactionIDOrErrorChan := make(chan RecurringTransactionIDOrError)
	go consumer.recurringTransactionIDFetcher.FetchID(document.Institution, document.InstititionEntityID, transactionIDOrErrorChan)

	transactionInstanceIDOrErrorChan := make(chan RecurringTransactionInstanceIDOrError)
	go consumer.recurringTransactionInstanceIDFetcher.FetchID(document.Institution, document.InstititionEntityID, transactionInstanceIDOrErrorChan)

	transactionIDOrError := <-transactionIDOrErrorChan
	transactionInstanceIDOrError := <-transactionInstanceIDOrErrorChan

	// TODO: propagate this error?
	if transactionInstanceIDOrError.err != nil {
		log.Printf("Could not find instance ID for direct debit: %v", transactionInstanceIDOrError.err)
	} else if transactionIDOrError.err != nil {
		log.Printf("Could not find recurring ID for direct debit: %v", transactionInstanceIDOrError.err)
	} else {
		transactionID := transactionInstanceIDOrError.ID
		recurringTransactionID := transactionIDOrError.ID

		cmd := ProcessDirectDebitTransactionDocumentCommand{
			RecurringTransactionID: *recurringTransactionID,
			TransactionID:          *transactionID,
			Institution:            document.Institution,
			InstititionEntityID:    document.InstititionEntityID,
			From:                   NewTransactionParty(&document.FromIBAN, nil),
			To:                     NewTransactionParty(document.ToIBAN, nil),
			TransactionDate:        document.TransactionDate,
			Amount:                 primitives.NewMoneyForCommand(document.Amount),
			FetchTimestamp:         document.FetchTimestamp,
		}
		consumer.handleCommand(cmd)
	}
}

func (consumer DocumentsFromBusConsumer) handleScheduleDocument(wg *sync.WaitGroup, document bus.ScheduleDocument) {
	defer wg.Done()

	transactionIDOrErrorChan := make(chan RecurringTransactionIDOrError)
	go consumer.recurringTransactionIDFetcher.FetchID(document.Institution, document.InstitutionEntityID, transactionIDOrErrorChan)

	transactionIDOrError := <-transactionIDOrErrorChan

	// TODO: propagate this error?
	if transactionIDOrError.err != nil {
		log.Printf("Could not find recurring ID for schedule: %v", transactionIDOrError.err)
	} else {
		recurringTransactionID := transactionIDOrError.ID

		cmd := ProcessScheduleCommand{
			RecurringTransactionID: *recurringTransactionID,
			Institution:            document.Institution,
			InstitutionEntityID:    document.InstitutionEntityID,
			FromIBAN:               document.FromIBAN,
			ToIBAN:                 document.ToIBAN,
			ToName:                 document.ToName,
			Frequency:              document.Frequency,
			StartDate:              document.StartDate,
			EndDate:                document.EndDate,
			Amount:                 primitives.NewMoneyForCommand(document.Amount),
			FetchTimestamp:         document.FetchTimestamp,
		}
		consumer.handleCommand(cmd)
	}
}

func (consumer DocumentsFromBusConsumer) handleCommand(cmd eh.Command) {
	if err := consumer.handler.HandleCommand(consumer.context, cmd); err != nil {
		handleCommandError(cmd, err)
	}
}

func handleCommandError(cmd eh.Command, err error) {
	switch err.(type) {
	case eh.CommandFieldError:
		if hasInvalidFields(cmd) {
			log.Printf("Could not handle command of type %s, error: %v\nThe command contains pointers, chans or functions types, that might be the problem?", utils.TypeNameOf(cmd), err)
			return
		}
	}

	log.Printf("Could not handle command of type %s, error: %v\n", utils.TypeNameOf(cmd), err)
}

func hasInvalidFields(cmd eh.Command) bool {
	rv := reflect.Indirect(reflect.ValueOf(cmd))
	rt := rv.Type()

	for i := 0; i < rt.NumField(); i++ {
		switch rv.Field(i).Kind() {
		case reflect.Func, reflect.Chan, reflect.Uintptr, reflect.Ptr, reflect.UnsafePointer:
			return true
		}
	}

	return false
}
