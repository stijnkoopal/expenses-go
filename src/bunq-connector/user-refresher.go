package bunqconnector

import (
	"app/bus"
	"app/primitives"
	"context"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
)

type userRefresher interface {
	refresh(userID primitives.UserID)
}

type userRefresherWithBusIntegration struct {
	context                    context.Context
	api                        bunqAPI
	refreshTimestampRepository refreshTimestampRepository
	busChannels                integrationChannels
}

func createUserRefresherWithBusIntegration(
	ctx context.Context,
	api bunqAPI,
	refreshTimestampRepository refreshTimestampRepository,
	busChannels integrationChannels,
) userRefresher {
	return userRefresherWithBusIntegration{
		context:                    ctx,
		api:                        api,
		refreshTimestampRepository: refreshTimestampRepository,
		busChannels:                busChannels,
	}
}

type accountToRefresh struct {
	apiAccount
	hasEverBeenRefreshed bool
	lastRefresh          *time.Time
	userID               primitives.UserID
}

func (refresher userRefresherWithBusIntegration) refresh(userID primitives.UserID) {
	accountsOrErrorsFromAPI := make(chan apiAccountOrError, 10)

	go refresher.api.fetchAccounts(refresher.context, accountsOrErrorsFromAPI)

	for {
		select {
		case <-refresher.context.Done():
			break

		case accountOrError, ok := <-accountsOrErrorsFromAPI:
			if !ok {
				return
			}

			if accountOrError.err != nil {
				log.Printf("Unknown error from api: %s", accountOrError.err)
			} else {
				go refresher.syncAccount(userID, accountOrError.apiAccount)
			}
		}
	}
}

func (refresher userRefresherWithBusIntegration) enrichAccount(userID primitives.UserID, account apiAccount, result chan<- accountToRefresh) {
	defer func() { close(result) }()

	fetchResult := make(chan fetchLastRefreshForResult)
	go refresher.refreshTimestampRepository.fetchLastRefreshFor(account.bunqAccountID, fetchResult)

	select {
	case <-refresher.context.Done():
		break
	case lastRefresh, ok := <-fetchResult:
		var withRefresh accountToRefresh
		if !ok || lastRefresh.err != nil {
			if lastRefresh.err != nil {
				log.Printf("Could not fetch auth for user because: %s", lastRefresh.err)
			}

			withRefresh = accountToRefresh{
				apiAccount:           account,
				userID:               userID,
				hasEverBeenRefreshed: false,
				lastRefresh:          nil,
			}
		} else {
			withRefresh = accountToRefresh{
				apiAccount:           account,
				userID:               userID,
				hasEverBeenRefreshed: lastRefresh.isEverBeenRefreshed,
				lastRefresh:          lastRefresh.lastRefresh,
			}
		}
		result <- withRefresh
	}
}

func (refresher userRefresherWithBusIntegration) syncAccount(userID primitives.UserID, account apiAccount) {
	enrichedAccount := make(chan accountToRefresh)
	go refresher.enrichAccount(userID, account, enrichedAccount)

	select {
	case <-refresher.context.Done():
		break

	case account, ok := <-enrichedAccount:
		if !ok {
			return
		}

		startUpdate := newStartRefreshUpdateFor(account)
		defer refresher.doneSyncing(account, startUpdate)

		refresher.busChannels.updatesChannel() <- startUpdate
		refresher.busChannels.accountChannel() <- account.mapToDocument(account.userID)

		wg := sync.WaitGroup{}
		wg.Add(3)

		// go refresher.syncTransactions(&wg, account)
		// go refresher.syncSchedules(&wg, account)
		go refresher.syncDirectDebits(&wg, account)

		wg.Wait()
	}
}

func (refresher userRefresherWithBusIntegration) doneSyncing(account accountToRefresh, startUpdate bus.StartRefreshUpdate) {
	defer func() { refresher.busChannels.updatesChannel() <- newDoneUpdate(startUpdate) }()

	select {
	case <-refresher.context.Done():
		break
	default:
		result := make(chan saveLastRefreshResult)
		go refresher.refreshTimestampRepository.saveLastRefresh(account.bunqAccountID, startUpdate.Started, result)

		for r := range result {
			if r.err != nil {
				log.Printf("Unable to save last refresh time for account %d, due to %s", account.bunqAccountID, r.err)
			}
		}
	}
}

func (refresher userRefresherWithBusIntegration) syncTransactions(wg *sync.WaitGroup, account accountToRefresh) {
	defer wg.Done()

	transactions := make(chan apiTransactionOrError, 50)

	var lastRefresh time.Time
	if account.hasEverBeenRefreshed {
		lastRefresh = *account.lastRefresh
	} else {
		lastRefresh = time.Unix(0, 0)
	}

	go refresher.api.fetchTransactions(refresher.context, account.bunqAccountID, lastRefresh, transactions)

	for {
		select {
		case <-refresher.context.Done():
			return

		case tx, ok := <-transactions:
			if !ok {
				return
			}

			if tx.err != nil {
				log.Printf("Error syncing tx: %s", tx.err)
			} else {
				refresher.busChannels.transactionChannel() <- tx.apiTransaction.mapToDocument()
			}
		}
	}
}

func (refresher userRefresherWithBusIntegration) syncSchedules(wg *sync.WaitGroup, account accountToRefresh) {
	defer wg.Done()

	schedules := make(chan apiScheduleOrError, 50)
	go refresher.api.fetchSchedules(refresher.context, account.bunqAccountID, schedules)

	for {
		select {
		case <-refresher.context.Done():
			return

		case schedule, ok := <-schedules:
			if !ok {
				return
			}

			if schedule.err != nil {
				log.Printf("Error syncing schedules: %s", schedule.err)
			} else {
				refresher.busChannels.scheduleChannel() <- schedule.apiSchedule.mapToDocument()
			}
		}
	}
}

func (refresher userRefresherWithBusIntegration) syncDirectDebits(wg *sync.WaitGroup, account accountToRefresh) {
	defer wg.Done()

	directDebits := make(chan apiDirectDebitTransactionOrError, 50)
	go refresher.api.fetchDirectDebitTransactions(refresher.context, account.bunqAccountID, directDebits)

	for {
		select {
		case <-refresher.context.Done():
			return

		case directDebit, ok := <-directDebits:
			if !ok {
				return
			}
			if directDebit.err != nil {
				log.Printf("Error syncing directDebit: %s", directDebit.err)
			} else {
				refresher.busChannels.directDebitChannel() <- directDebit.apiDirectDebitTransaction.mapToDocument()
			}
		}
	}
}

func newStartRefreshUpdateFor(account accountToRefresh) bus.StartRefreshUpdate {
	return bus.StartRefreshUpdate{
		UserID:              account.userID,
		InstitutionEntityID: strconv.Itoa(int(account.bunqAccountID)),
		SyncID:              primitives.SyncID(uuid.New()),
		Started:             time.Now(),
	}
}
