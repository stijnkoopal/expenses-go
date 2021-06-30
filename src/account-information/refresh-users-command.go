package accountinformation

import (
	"app/primitives"
	"context"
	"fmt"

	"github.com/google/uuid"
)

type RefreshUsersCommand interface {
	StartRefresh()
}

type fetchUserIdsResult struct {
	userID *primitives.UserID
	err    error
}

type usersRepository interface {
	FetchUserIds(out chan<- fetchUserIdsResult)
}

type inMemoryUserRepository struct {
}

func NewInMemoryUserRepository() inMemoryUserRepository {
	return inMemoryUserRepository{}
}

func (repo inMemoryUserRepository) FetchUserIds(out chan<- fetchUserIdsResult) {
	userID := primitives.UserID(uuid.New())
	out <- fetchUserIdsResult{userID: &userID}
	close(out)
}

type RefreshUsersFromRepositoryCommand struct {
	context            context.Context
	usersRepository    usersRepository
	refreshUserCommand StartUserRefreshCommand
}

type StartUserRefreshCommand interface {
	Refresh(userID primitives.UserID)
}

func NewRefreshUsersFromRepositoryCommand(ctx context.Context, repo usersRepository, refreshUserCommand StartUserRefreshCommand) RefreshUsersFromRepositoryCommand {
	return RefreshUsersFromRepositoryCommand{
		context:            ctx,
		usersRepository:    repo,
		refreshUserCommand: refreshUserCommand,
	}
}

func (cmd RefreshUsersFromRepositoryCommand) StartRefresh() {
	userIDs := make(chan fetchUserIdsResult, 10)
	go cmd.usersRepository.FetchUserIds(userIDs)

	for {
		select {
		case <-cmd.context.Done():
			return
		case userID, ok := <-userIDs:
			if !ok {
				return
			}

			if userID.err != nil {
				// TODO:
				fmt.Println(userID.err)
			} else {
				go cmd.refreshUserCommand.Refresh(*userID.userID)
			}
		}
	}
}
