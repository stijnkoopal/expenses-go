package bunqconnector

import (
	"app/primitives"
	"fmt"
	"os"

	"github.com/google/uuid"
)

type authID uuid.UUID

func (authID authID) String() string {
	return uuid.UUID(authID).String()
}

type auth struct {
	id         authID
	userID     primitives.UserID
	apiContext string
	bunqUserID string
}

type authResult struct {
	result *auth
	err    error
}

type deleteAuthResult struct {
	err error
}

type authRepository interface {
	fetchAuthsForUser(userID primitives.UserID, out chan<- authResult)
	deleteAuth(userID primitives.UserID, id authID, out chan<- deleteAuthResult)
}

type postgresAuthRepository struct{}

func newPostgresAuthRepository() postgresAuthRepository {
	return postgresAuthRepository{}
}

func (repo postgresAuthRepository) fetchAuthsForUser(userID primitives.UserID, out chan<- authResult) {
	fmt.Fprintf(os.Stdout, "Fetching auth for %s\n", userID)
	out <- authResult{result: &auth{}, err: nil}
	close(out)
}

func (repo postgresAuthRepository) deleteAuth(userID primitives.UserID, id authID, out chan<- deleteAuthResult) {
	fmt.Fprintf(os.Stdout, "Deleting auth for %s\n", userID)
	out <- deleteAuthResult{}
	close(out)
}

type inMemoryAuthRepository struct{}

func newInMemoryAuthRepository() inMemoryAuthRepository {
	return inMemoryAuthRepository{}
}

func (repo inMemoryAuthRepository) fetchAuthsForUser(userID primitives.UserID, out chan<- authResult) {
	out <- authResult{result: &auth{
		id:         authID(uuid.New()),
		bunqUserID: "",
		userID:     primitives.UserID(uuid.New()),
		apiContext: `
			`,
	}}
	close(out)
}

func (repo inMemoryAuthRepository) deleteAuth(userID primitives.UserID, id authID, out chan<- deleteAuthResult) {
	out <- deleteAuthResult{}
	close(out)
}
