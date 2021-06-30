package accountinformation

import (
	"app/bus"
	"app/primitives"
	"app/utils"
	"context"
	"fmt"
	"log"
	"reflect"
	"sync"

	"github.com/jinzhu/copier"

	eh "github.com/looplab/eventhorizon"
)

type DocumentsConsumer interface {
	Start()
}

type DocumentsFromBusConsumer struct {
	context                  context.Context
	handler                  eh.CommandHandler
	monetaryAccountIDFetcher MonetaryAccountIDFetcher
	TransactionIDFetcher     TransactionIDFetcher
}

func NewDocumentsFromBusConsumer(
	ctx context.Context,
	handler eh.CommandHandler,
	monetaryAccountIDFetcher MonetaryAccountIDFetcher,
	transactionIDFetcher TransactionIDFetcher,
) DocumentsConsumer {
	return DocumentsFromBusConsumer{context: ctx, handler: handler, monetaryAccountIDFetcher: monetaryAccountIDFetcher, TransactionIDFetcher: transactionIDFetcher}
}

func (consumer DocumentsFromBusConsumer) Start() {
	wg := sync.WaitGroup{}

	accountChannel := bus.AccountChannelForReading()
	transactionChannel := bus.TransactionChannelForReading()

	for {
		select {
		case document, ok := <-accountChannel:
			if !ok {
				accountChannel = nil
			} else {
				wg.Add(1)
				go consumer.handleAccountDocument(&wg, document)
			}

		case document, ok := <-transactionChannel:
			if !ok {
				transactionChannel = nil
			} else {
				wg.Add(1)
				go consumer.handleTransactionDocument(&wg, document)
			}
		}

		if accountChannel == nil && transactionChannel == nil {
			break
		}
	}

	wg.Wait()
}

func (consumer DocumentsFromBusConsumer) handleAccountDocument(wg *sync.WaitGroup, document bus.MonetaryAccountDocument) {
	defer wg.Done()

	idOrErrorChan := make(chan MonetaryAccountIDOrError)
	go consumer.monetaryAccountIDFetcher.FetchID(&document.Iban, &document.Institution, &document.InstitutionEntityID, &document.Alias, idOrErrorChan)

	idOrError := <-idOrErrorChan

	// TODO: propagate this error?
	if idOrError.err != nil {
		log.Printf("Could not find ID for monetary account: %v", idOrError.err)
	} else {
		monetaryAccountID := idOrError.ID
		cmd := ProcessMonetaryAccountCommand{
			MonetaryAccountID:   *monetaryAccountID,
			Iban:                document.Iban,
			Joint:               document.Joint,
			OwnerUserID:         document.OwnerUserID,
			Alias:               document.Alias,
			Institution:         document.Institution,
			InstitutionEntityID: document.InstitutionEntityID,
			Balance:             primitives.NewMoneyForCommand(document.Balance),
			FetchTimestamp:      document.FetchTimestamp,
		}
		consumer.handleCommand(cmd)
	}
}

func (consumer DocumentsFromBusConsumer) handleTransactionDocument(wg *sync.WaitGroup, document bus.TransactionDocument) {
	defer wg.Done()

	fromIDOrErrorChan := make(chan MonetaryAccountIDOrError)
	toIDOrErrorChan := make(chan MonetaryAccountIDOrError)
	transaactionIDOrErrorChan := make(chan TransactionIDOrError)

	go consumer.monetaryAccountIDFetcher.FetchID(document.FromIBAN, document.FromInstition, document.FromInstitutionEntityID, document.FromName, fromIDOrErrorChan)
	go consumer.monetaryAccountIDFetcher.FetchID(document.ToIBAN, document.ToInstition, document.ToInstitionEntityID, document.ToName, toIDOrErrorChan)
	go consumer.TransactionIDFetcher.FetchID(document.FromIBAN, document.ToIBAN, document.Amount, document.Description, document.TransactionDate, transaactionIDOrErrorChan)

	fromIDOrError := <-fromIDOrErrorChan
	toIDOrError := <-toIDOrErrorChan
	transactionIDOrError := <-transaactionIDOrErrorChan

	// TODO: propagate these errors?
	if fromIDOrError.err != nil {
		log.Printf("Could not find ID for monetary account: %v", fromIDOrError.err)
	} else if toIDOrError.err != nil {
		log.Printf("Could not find ID for monetary account: %v", toIDOrError.err)
	} else if transactionIDOrError.err != nil {
		log.Printf("Could not find ID for transaction: %v", transactionIDOrError.err)
	} else {
		fromMonetaryAccountID := fromIDOrError.ID
		toMonetaryAccountID := toIDOrError.ID
		transactionID := transactionIDOrError.ID

		baseCommand := ProcessTransactionDocumentCommand{
			ID:                    *transactionID,
			Amount:                primitives.NewMoneyForCommand(document.Amount),
			From:                  NewTransactionParty(document.FromIBAN, document.FromName),
			FromMonetaryAccountID: *fromMonetaryAccountID,
			To:                    NewTransactionParty(document.ToIBAN, document.ToName),
			ToMonetaryAccountID:   *toMonetaryAccountID,
			Description:           document.Description,
			InstitutionScheduleID: utils.EmptyStringOrValue(document.InstitutionScheduleID),
			IsScheduled:           document.InstitutionScheduleID != nil,
			BalanceAfterMutation:  primitives.NewMoneyForCommand(document.BalanceAfterMutation),
			TransactionDate:       document.TransactionDate,
			FetchTimestamp:        document.FetchTimestamp,
		}

		if document.FromInstitutionEntityID != nil {
			baseCommand.InstitutionEntityID = *document.FromInstitutionEntityID
		} else {
			baseCommand.InstitutionEntityID = *document.ToInstitionEntityID
		}

		cmd1 := ProcessTransactionDocumentCommand{}
		err := copier.Copy(&cmd1, baseCommand)
		if err != nil {
			fmt.Println(err)
			return
		}
		cmd1.MonetaryAccountID = *fromMonetaryAccountID
		consumer.handleCommand(cmd1)

		cmd2 := ProcessTransactionDocumentCommand{}
		copier.Copy(&cmd2, baseCommand)
		cmd2.MonetaryAccountID = *toMonetaryAccountID
		consumer.handleCommand(cmd2)
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
