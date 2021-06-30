package bunqconnector

import "time"

type fetchLastRefreshForResult struct {
	lastRefresh         *time.Time
	isEverBeenRefreshed bool
	err                 error
}

type saveLastRefreshResult struct {
	err error
}

type refreshTimestampRepository interface {
	fetchLastRefreshFor(accountID bunqAccountID, out chan<- fetchLastRefreshForResult)
	saveLastRefresh(accountID bunqAccountID, lastRefresh time.Time, out chan<- saveLastRefreshResult)
}

type postgresRefreshTimestampRepository struct{}

func newPostgresRefreshTimestampRepository() postgresRefreshTimestampRepository {
	return postgresRefreshTimestampRepository{}
}

func (repo postgresRefreshTimestampRepository) fetchLastRefreshFor(accountID bunqAccountID, out chan<- fetchLastRefreshForResult) {

}

func (repo postgresRefreshTimestampRepository) saveLastRefresh(accountID bunqAccountID, lastRefresh time.Time, out chan<- saveLastRefreshResult) {

}

type inMemoryRefreshTimestampRepository struct {
	timestamps map[bunqAccountID]time.Time
}

func newInMemoryRefreshTimestampRepository() inMemoryRefreshTimestampRepository {
	return inMemoryRefreshTimestampRepository{
		timestamps: make(map[bunqAccountID]time.Time),
	}
}

func (repo inMemoryRefreshTimestampRepository) fetchLastRefreshFor(accountID bunqAccountID, out chan<- fetchLastRefreshForResult) {
	lastRefresh, ok := repo.timestamps[accountID]
	if !ok {
		lastRefresh = time.Unix(0, 0)
	}
	out <- fetchLastRefreshForResult{lastRefresh: &lastRefresh, isEverBeenRefreshed: ok}
	close(out)
}

func (repo inMemoryRefreshTimestampRepository) saveLastRefresh(accountID bunqAccountID, lastRefresh time.Time, out chan<- saveLastRefreshResult) {
	repo.timestamps[accountID] = lastRefresh
	out <- saveLastRefreshResult{}
	close(out)
}
