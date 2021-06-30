package bunqconnector

import (
	"app/bus"
	"app/primitives"
	"context"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/goleak"
)

type fakeBunqAPI struct {
	mock.Mock
}

func (m *fakeBunqAPI) fetchAccounts(ctx context.Context, out chan<- apiAccountOrError) {
	m.Called(ctx, out)
}

func (m *fakeBunqAPI) fetchTransactions(ctx context.Context, bunqAccountID bunqAccountID, newerThan time.Time, out chan<- apiTransactionOrError) {
	m.Called(ctx, bunqAccountID, newerThan, out)
}

func (m *fakeBunqAPI) fetchDirectDebitTransactions(ctx context.Context, bunqAccountID bunqAccountID, out chan<- apiDirectDebitTransactionOrError) {
	m.Called(ctx, bunqAccountID, out)
}

func (m *fakeBunqAPI) fetchSchedules(ctx context.Context, bunqAccountID bunqAccountID, out chan<- apiScheduleOrError) {
	m.Called(ctx, bunqAccountID, out)
}

type fakeAuthRepository struct {
	mock.Mock
}

func (m *fakeAuthRepository) fetchAuthsForUser(userID primitives.UserID, out chan<- authResult) {
	m.Called(userID, out)
}

func (m *fakeAuthRepository) deleteAuth(userID primitives.UserID, id authID, out chan<- deleteAuthResult) {
	m.Called(userID, id, out)
}

type fakeRefreshTimestampRepository struct {
	mock.Mock
}

func (m *fakeRefreshTimestampRepository) fetchLastRefreshFor(accountID bunqAccountID, out chan<- fetchLastRefreshForResult) {
	m.Called(accountID, out)
}

func (m *fakeRefreshTimestampRepository) saveLastRefresh(accountID bunqAccountID, lastRefresh time.Time, out chan<- saveLastRefreshResult) {
	m.Called(accountID, out)
}

type fakeIntegrationChannels struct {
	mock.Mock
}

func (m *fakeIntegrationChannels) updatesChannel() chan<- bus.Update {
	args := m.Called()
	return args.Get(0).(chan bus.Update)
}

func (m *fakeIntegrationChannels) accountChannel() chan<- bus.MonetaryAccountDocument {
	args := m.Called()
	return args.Get(0).(chan bus.MonetaryAccountDocument)
}

func (m *fakeIntegrationChannels) transactionChannel() chan<- bus.TransactionDocument {
	args := m.Called()
	return args.Get(0).(chan bus.TransactionDocument)
}

func (m *fakeIntegrationChannels) scheduleChannel() chan<- bus.ScheduleDocument {
	args := m.Called()
	return args.Get(0).(chan bus.ScheduleDocument)
}

func (m *fakeIntegrationChannels) directDebitChannel() chan<- bus.DirectDebitTransactionDocument {
	args := m.Called()
	return args.Get(0).(chan bus.DirectDebitTransactionDocument)
}

type fakeAPIFactory struct {
	api bunqAPI
}

func (f fakeAPIFactory) factory(ctx context.Context, limiter rateLimiter, contextJSON string) bunqAPIOrError {
	return bunqAPIOrError{bunqAPI: f.api}
}

type testScope struct {
	ctx                        context.Context
	ctxCancel                  context.CancelFunc
	userID                     primitives.UserID
	api                        *fakeBunqAPI
	authRepository             *fakeAuthRepository
	refreshTimestampRepository *fakeRefreshTimestampRepository
	channels                   *fakeIntegrationChannels
	cmd                        StartUserRefreshCommand

	updatesBus     chan bus.Update
	accountsBus    chan bus.MonetaryAccountDocument
	transactionBus chan bus.TransactionDocument
	scheduleBus    chan bus.ScheduleDocument
	directDebitBus chan bus.DirectDebitTransactionDocument
}

func newTestScope() *testScope {
	s := new(testScope)

	ctx, cancel := context.WithCancel(context.Background())
	s.ctx = ctx
	s.ctxCancel = cancel
	s.userID = primitives.UserID(uuid.New())
	s.api = new(fakeBunqAPI)
	s.authRepository = new(fakeAuthRepository)
	s.refreshTimestampRepository = new(fakeRefreshTimestampRepository)
	s.channels = new(fakeIntegrationChannels)
	s.cmd = StartUserRefreshCommand{
		limiter:                    newDefaultRateLimiter(s.ctx),
		refreshTimestampRepository: s.refreshTimestampRepository,
		authRepository:             s.authRepository,
		channels:                   s.channels,
		apiFactory:                 fakeAPIFactory{api: s.api},
		context:                    s.ctx,
	}

	s.updatesBus = make(chan bus.Update)
	s.channels.On("updatesChannel").Return(s.updatesBus)

	s.accountsBus = make(chan bus.MonetaryAccountDocument)
	s.channels.On("accountChannel").Return(s.accountsBus)

	s.transactionBus = make(chan bus.TransactionDocument)
	s.channels.On("transactionChannel").Return(s.transactionBus)

	s.scheduleBus = make(chan bus.ScheduleDocument)
	s.channels.On("scheduleChannel").Return(s.scheduleBus)

	s.directDebitBus = make(chan bus.DirectDebitTransactionDocument)
	s.channels.On("directDebitChannel").Return(s.directDebitBus)

	return s
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func Test_UserRefresher_Refresh_ShouldDoNothingWhenNoAuths(t *testing.T) {
	s := newTestScope()
	s.authRepository.On("fetchAuthsForUser", s.userID, mock.Anything).Run(func(args mock.Arguments) {
		close(args.Get(1).(chan<- authResult))
	})

	s.cmd.Refresh(s.userID)

	s.authRepository.AssertExpectations(t)
	s.ctxCancel()
}

func Test_UserRefresher_Refresh_ShouldBeDoneRefreshingWhenContextCancelling(t *testing.T) {
	s := newTestScope()
	bunqID := bunqAccountID(12)

	s.authRepository.On("fetchAuthsForUser", s.userID, mock.Anything).Run(func(args mock.Arguments) {
		channel := args.Get(1).(chan<- authResult)
		channel <- authResult{result: &auth{}}
		close(channel)
	})

	s.api.On("fetchAccounts", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		channel := args.Get(1).(chan<- apiAccountOrError)
		channel <- apiAccountOrError{apiAccount: apiAccount{bunqAccountID: bunqID}}
		close(channel)
	})

	s.refreshTimestampRepository.On("fetchLastRefreshFor", bunqID, mock.Anything).Run(func(args mock.Arguments) {
		channel := args.Get(1).(chan<- fetchLastRefreshForResult)
		close(channel)
	})

	s.api.On("fetchDirectDebitTransactions", mock.Anything, bunqID, mock.Anything)
	s.api.On("fetchTransactions", mock.Anything, bunqID, time.Unix(0, 0), mock.Anything)
	s.api.On("fetchSchedules", mock.Anything, bunqID, mock.Anything)

	s.cmd.Refresh(s.userID)

	startUpdateReceived := false
	doneUpdatingReceived := false

	for {
		select {
		case <-s.accountsBus:
			s.accountsBus = nil
		case x := <-s.updatesBus:
			switch x.(type) {
			case bus.StartRefreshUpdate:
				startUpdateReceived = true

			case bus.DoneRefreshingUpdate:
				doneUpdatingReceived = true
				s.updatesBus = nil
			}
		}

		s.ctxCancel()

		if startUpdateReceived &&
			doneUpdatingReceived {
			break
		}
	}

	s.authRepository.AssertExpectations(t)
	s.refreshTimestampRepository.AssertExpectations(t)
}

func Test_UserRefresher_Refresh_ShouldRefreshOneAccountWithOneAuth(t *testing.T) {
	s := newTestScope()
	bunqID := bunqAccountID(12)

	s.authRepository.On("fetchAuthsForUser", s.userID, mock.Anything).Run(func(args mock.Arguments) {
		channel := args.Get(1).(chan<- authResult)
		channel <- authResult{result: &auth{}}
		close(channel)
	})

	s.api.On("fetchAccounts", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		channel := args.Get(1).(chan<- apiAccountOrError)
		channel <- apiAccountOrError{apiAccount: apiAccount{bunqAccountID: bunqID}}
		close(channel)
	})

	s.refreshTimestampRepository.On("fetchLastRefreshFor", bunqID, mock.Anything).Run(func(args mock.Arguments) {
		channel := args.Get(1).(chan<- fetchLastRefreshForResult)
		close(channel)
	})

	s.api.On("fetchDirectDebitTransactions", mock.Anything, bunqID, mock.Anything).Run(func(args mock.Arguments) {
		channel := args.Get(2).(chan<- apiDirectDebitTransactionOrError)
		channel <- apiDirectDebitTransactionOrError{}
		close(channel)
	})

	s.api.On("fetchTransactions", mock.Anything, bunqID, time.Unix(0, 0), mock.Anything).Run(func(args mock.Arguments) {
		channel := args.Get(3).(chan<- apiTransactionOrError)
		channel <- apiTransactionOrError{}
		close(channel)
	})

	s.api.On("fetchSchedules", mock.Anything, bunqID, mock.Anything).Run(func(args mock.Arguments) {
		channel := args.Get(2).(chan<- apiScheduleOrError)
		channel <- apiScheduleOrError{}
		close(channel)
	})

	s.refreshTimestampRepository.On("saveLastRefresh", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		channel := args.Get(1).(chan<- saveLastRefreshResult)
		channel <- saveLastRefreshResult{}
		close(channel)
	})

	go s.cmd.Refresh(s.userID)

	accountDocumentReceived := false
	transactionDocumentReceived := false
	scheduleDocumentReceived := false
	directDebitDocumentReceived := false
	startUpdateReceived := false
	doneUpdatingReceived := false

	for {
		select {
		case accountDocument := <-s.accountsBus:
			assert.Equal(t, strconv.Itoa(int(bunqID)), accountDocument.InstitutionEntityID)
			accountDocumentReceived = true
			s.accountsBus = nil
		case <-s.transactionBus:
			transactionDocumentReceived = true
			s.transactionBus = nil
		case <-s.scheduleBus:
			scheduleDocumentReceived = true
			s.scheduleBus = nil
		case <-s.directDebitBus:
			directDebitDocumentReceived = true
			s.directDebitBus = nil
		case x := <-s.updatesBus:
			switch x.(type) {
			case bus.StartRefreshUpdate:
				startUpdateReceived = true
			case bus.DoneRefreshingUpdate:
				doneUpdatingReceived = true
				s.updatesBus = nil
			}
		}

		if accountDocumentReceived &&
			transactionDocumentReceived &&
			scheduleDocumentReceived &&
			directDebitDocumentReceived &&
			startUpdateReceived &&
			doneUpdatingReceived {
			break
		}
	}

	s.authRepository.AssertExpectations(t)
	s.refreshTimestampRepository.AssertExpectations(t)
	s.api.AssertExpectations(t)
	s.ctxCancel()
}

func typeNameOf(v interface{}) string {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface {
		rv = rv.Elem()
	}

	if rv.IsValid() {
		return rv.Type().Name()
	}
	return "Invalid"
}

func toSlice(ctx context.Context, c chan interface{}) []interface{} {
	s := make([]interface{}, 0)
	for i := range c {
		s = append(s, i)
	}
	return s
}
