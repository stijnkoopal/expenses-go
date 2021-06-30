package bunqconnector

import (
	"app/primitives"
	"context"
	"encoding/json"
	"fmt"

	"github.com/OGKevin/go-bunq/bunq"
)

type apiFactory interface {
	factory(ctx context.Context, limiter rateLimiter, contextJSON string) bunqAPIOrError
}

// StartUserRefreshCommand allows the caller to refresh accounts, transactions, schedules and direct debits from all bunq accounts
type StartUserRefreshCommand struct {
	limiter                    rateLimiter
	refreshTimestampRepository refreshTimestampRepository
	authRepository             authRepository
	channels                   integrationChannels
	apiFactory                 apiFactory
	context                    context.Context
}

// NewStartUserRefreshCommand creates a new StartUserRefreshCommand with bunq production values
func NewStartUserRefreshCommand(ctx context.Context) StartUserRefreshCommand {
	cmd := new(StartUserRefreshCommand)
	cmd.limiter = newDefaultRateLimiter(ctx)
	cmd.refreshTimestampRepository = newInMemoryRefreshTimestampRepository()
	cmd.authRepository = newInMemoryAuthRepository()
	cmd.channels = newBusChannels()
	cmd.apiFactory = bunqAPIFactory{}
	cmd.context = ctx
	return *cmd
}

// Refresh starts the refresh of the given user
func (cmd StartUserRefreshCommand) Refresh(userID primitives.UserID) {
	auths := make(chan authResult, 5)
	go cmd.authRepository.fetchAuthsForUser(userID, auths)

	for {
		select {
		case <-cmd.context.Done():
			break

		case auth, ok := <-auths:
			if !ok {
				return
			}

			if auth.err != nil {
				// TODO
				fmt.Println(auth.err)
			} else {
				go cmd.startRefreshForAuth(userID, auth.result)
			}
		}
	}
}

func (cmd StartUserRefreshCommand) startRefreshForAuth(userID primitives.UserID, auth *auth) {
	clients := make(chan bunqAPIOrError)
	go cmd.buildAPI(auth.apiContext, clients)

	select {
	case <-cmd.context.Done():
		return
	case client := <-clients:
		if client.err != nil {
			fmt.Printf("Removing auth %s for user %s, api error: %v\n", auth.id, userID, client.err)

			result := make(chan deleteAuthResult)
			cmd.authRepository.deleteAuth(userID, auth.id, result)

			go func() {
				select {
				case <-cmd.context.Done():
				case r := <-result:
					if r.err != nil {
						fmt.Printf("Could not remove auth, err: %v\n", r.err)
					}
				}
			}()

		} else {
			refresher := createUserRefresherWithBusIntegration(cmd.context, client.bunqAPI, cmd.refreshTimestampRepository, cmd.channels)
			go refresher.refresh(userID)
		}
	}
}

type bunqAPIOrError struct {
	bunqAPI
	err error
}

func (cmd StartUserRefreshCommand) buildAPI(apiContextJSON string, out chan<- bunqAPIOrError) {
	out <- cmd.apiFactory.factory(cmd.context, cmd.limiter, apiContextJSON)
}

type bunqAPIFactory struct{}

func (f bunqAPIFactory) factory(ctx context.Context, limiter rateLimiter, contextJSON string) bunqAPIOrError {
	var bunqContext *bunq.ClientContext
	if err := json.Unmarshal([]byte(contextJSON), &bunqContext); err != nil {
		return bunqAPIOrError{err: err}
	}

	client, err := bunq.NewClientFromContext(ctx, bunqContext)

	if err != nil {
		return bunqAPIOrError{err: err}
	}

	if err = client.Init(); err != nil {
		return bunqAPIOrError{err: err}
	}

	api := newRealBunqAPI(limiter, client)
	return bunqAPIOrError{bunqAPI: api}
}

/*
TO CREATE CONTEXT:

// key, err := bunq.CreateNewKeyPair()
// if err != nil {
// 	panic(err)
// }

// client := bunq.NewClient(context.Background(), bunq.BaseURLProduction, key, "", "Expenses")
// if err = client.Init(); err != nil {
// 	panic(err)
// }

// exportedContext, err := client.ExportClientContext()
// if err != nil {
// 	panic(err)
// }

// jsonBytes, err := json.MarshalIndent(exportedContext, "", "  ")
// if err != nil {
// 	panic(err)
// }
// fmt.Println(string(jsonBytes))
*/
