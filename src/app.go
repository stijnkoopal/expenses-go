package main

import (
	accountinformation "app/account-information"
	bunqconnector "app/bunq-connector"
	"app/bus"
	graphqladapter "app/graphql-adapter"
	"app/primitives"
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/Rhymond/go-money"

	"github.com/almerlucke/go-iban/iban"
	"github.com/google/uuid"

	"github.com/gorilla/mux"
)

type fakeStartUserRefreshCommand struct {
	ctx            context.Context
	updatesChannel chan<- bus.Update
	accountChannel chan<- bus.MonetaryAccountDocument
}

func newFakeStartUserRefreshCommand(ctx context.Context, updatesChannel chan<- bus.Update, accountChannel chan<- bus.MonetaryAccountDocument) fakeStartUserRefreshCommand {
	r := new(fakeStartUserRefreshCommand)
	r.ctx = ctx
	r.updatesChannel = updatesChannel
	r.accountChannel = accountChannel
	return *r
}

func (cmd fakeStartUserRefreshCommand) Refresh(userID primitives.UserID) {
	ticker := time.NewTicker(1000 * time.Millisecond)

	go func() {
		for {
			select {
			case <-cmd.ctx.Done():
				return

			case <-ticker.C:
				iban, _ := iban.NewIBAN("")
				user, _ := uuid.FromBytes([]byte("69359037-9599-48e7-b8f2-48393c019135"))
				cmd.accountChannel <- bus.MonetaryAccountDocument{
					Iban:                *iban,
					Joint:               false,
					OwnerUserID:         primitives.UserID(user),
					Alias:               "d",
					Institution:         primitives.Bunq,
					InstitutionEntityID: "x",
					Balance:             *money.New(1200, "EUR"),
					FetchTimestamp:      time.Now(),
				}
			}
		}
	}()
}

func main() {
	osSignalled := make(chan os.Signal, 1)
	signal.Notify(osSignalled, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())

	usersRepository := accountinformation.NewInMemoryUserRepository()

	startUserRefreshCommand := bunqconnector.NewStartUserRefreshCommand(ctx)
	// startUserRefreshCommand := newFakeStartUserRefreshCommand(ctx, bus.UpdatesChannelForWriting(), bus.AccountChannelForWriting())
	refreshUsersCommand := accountinformation.NewRefreshUsersFromRepositoryCommand(ctx, usersRepository, startUserRefreshCommand)

	go func() {
		<-osSignalled
		cancel()
	}()

	go refreshUsersCommand.StartRefresh()

	handler, err := NewHandler()
	if err != nil {
		log.Println(err)
	}

	monetaryAccountIDFetcher := accountinformation.NewInMemoryMonetaryAccountIDFetcher()
	transactionIDFetcher := accountinformation.NewInMemoryTransactionIDFetcher()
	documentsConsumer := accountinformation.NewDocumentsFromBusConsumer(ctx, handler.CommandHandler, monetaryAccountIDFetcher, transactionIDFetcher)
	go documentsConsumer.Start()

	muxes := make([]func(r *mux.Router) error, 3)
	muxes[0] = registerHealthchecks
	muxes[1] = graphqladapter.RegisterGraphql
	muxes[2] = bunqconnector.RegisterOAuthController

	if err := ServeHttp(ctx, muxes); err != nil {
		log.Printf("failed to serve:+%v\n", err)
	}
}
