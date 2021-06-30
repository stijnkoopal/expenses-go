package accountinformation

import (
	"app/primitives"
	"fmt"
	"strings"
	"sync"

	"github.com/jinzhu/copier"

	"github.com/almerlucke/go-iban/iban"
	"github.com/google/uuid"
)

var staticIds = make(map[iban.IBAN]primitives.MonetaryAccountID)

func init() {

}

func addEntry(m map[iban.IBAN]primitives.MonetaryAccountID, ibanString string, idString string) map[iban.IBAN]primitives.MonetaryAccountID {
	id, _ := uuid.FromBytes([]byte(idString))
	iba, _ := iban.NewIBAN(ibanString)

	res := make(map[iban.IBAN]primitives.MonetaryAccountID)
	copier.Copy(&res, m)
	res[*iba] = primitives.MonetaryAccountID(id)
	return res
}

type MonetaryAccountIDOrError struct {
	ID  *primitives.MonetaryAccountID
	err error
}

type MonetaryAccountIDFetcher interface {
	FetchID(iban *iban.IBAN, institution *primitives.Institution, institutionEntityID *string, alias *string, out chan<- MonetaryAccountIDOrError)
}

type InMemoryMonetaryAccountIDFetcher struct {
	mu                            sync.RWMutex
	accountIDsByIBAN              map[iban.IBAN]primitives.MonetaryAccountID
	accountIDsByInstitionEntityID map[string]primitives.MonetaryAccountID
	accountIDsByAlias             map[string]primitives.MonetaryAccountID
}

func NewInMemoryMonetaryAccountIDFetcher() *InMemoryMonetaryAccountIDFetcher {
	return &InMemoryMonetaryAccountIDFetcher{
		accountIDsByIBAN:              make(map[iban.IBAN]primitives.MonetaryAccountID),
		accountIDsByInstitionEntityID: make(map[string]primitives.MonetaryAccountID),
		accountIDsByAlias:             make(map[string]primitives.MonetaryAccountID),
	}
}

func (fetcher *InMemoryMonetaryAccountIDFetcher) FetchID(iban *iban.IBAN, institution *primitives.Institution, institutionEntityID *string, alias *string, out chan<- MonetaryAccountIDOrError) {
	defer close(out)

	var id *primitives.MonetaryAccountID
	var hasID bool
	var err error

	id, hasID = fetcher.read(iban, institutionEntityID, alias)

	if !hasID {
		id, err = fetcher.writeNew(iban, institutionEntityID, alias)

		if err != nil {
			out <- MonetaryAccountIDOrError{err: err}
			return
		}
	}

	out <- MonetaryAccountIDOrError{ID: id}
}

func (fetcher *InMemoryMonetaryAccountIDFetcher) read(iban *iban.IBAN, institutionEntityID *string, alias *string) (*primitives.MonetaryAccountID, bool) {
	fetcher.mu.RLock()
	defer fetcher.mu.RUnlock()

	if iban != nil {
		id, ok := fetcher.accountIDsByIBAN[*iban]
		if ok {
			return &id, true
		}
		id, ok = staticIds[*iban]
		if ok {
			return &id, true
		}
	}

	if institutionEntityID != nil {
		id, ok := fetcher.accountIDsByInstitionEntityID[*institutionEntityID]
		if ok {
			return &id, true
		}
	}

	if alias != nil {
		id, ok := fetcher.accountIDsByInstitionEntityID[normalizeAlias(*alias)]
		if ok {
			return &id, true
		}
	}

	return nil, false
}

func (fetcher *InMemoryMonetaryAccountIDFetcher) writeNew(iban *iban.IBAN, institutionEntityID *string, alias *string) (*primitives.MonetaryAccountID, error) {
	fetcher.mu.Lock()
	defer fetcher.mu.Unlock()

	id := primitives.MonetaryAccountID(uuid.New())
	if iban != nil {
		fetcher.accountIDsByIBAN[*iban] = id
		return &id, nil
	} else if institutionEntityID != nil {
		fetcher.accountIDsByInstitionEntityID[*institutionEntityID] = id
		return &id, nil
	} else if alias != nil {
		fetcher.accountIDsByAlias[normalizeAlias(*alias)] = id
		return &id, nil
	}
	return nil, fmt.Errorf("No way to fetch an id with no data bro")
}

func normalizeAlias(alias string) string {
	return strings.ToLower(alias)
}
