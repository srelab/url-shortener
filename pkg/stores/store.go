// Package stores provides support to interact with the entries
package stores

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"math/big"
	"strings"
	"time"
	"unicode"

	"github.com/srelab/url-shortener/pkg/logger"
	"github.com/srelab/url-shortener/pkg/util"

	"github.com/asaskevich/govalidator"
	"github.com/pborman/uuid"
	"github.com/pkg/errors"
	"github.com/srelab/url-shortener/pkg/g"
	"github.com/srelab/url-shortener/pkg/stores/redis"
	"github.com/srelab/url-shortener/pkg/stores/shared"
	"golang.org/x/crypto/bcrypt"
)

// Store holds internal funcs and vars about the store
type Store struct {
	storage  shared.Storage
	idLength int
}

// ErrNoValidURL is returned when the URL is not valid
var ErrNoValidURL = errors.New("the given URL is no valid URL")

// ErrGeneratingIDFailed is returned when the 10 tries to generate an id failed
var ErrGeneratingIDFailed = errors.New("could not generate unique id, all ten tries failed")

// ErrEntryIsExpired is returned when the entry is expired
var ErrEntryIsExpired = errors.New("entry is expired")

// New initializes the store with the db
func New() (*Store, error) {
	var err error
	var storage shared.Storage

	switch backend := g.GetConfig().Backend; backend {
	case "redis":
		conf := g.GetConfig().Redis
		storage, err = redis.New(conf.Host, conf.Password, conf.DB, conf.MaxRetries, conf.ReadTimeout, conf.WriteTimeout)

	//	TODO: badger key value db support
	//case "more storage implement":
	default:
		return nil, errors.New(backend + " is not a recognized backend")
	}

	if err != nil {
		return nil, errors.Wrap(err, "could not initialize the data backend")
	}

	return &Store{
		storage:  storage,
		idLength: g.GetConfig().ShortedIDLength,
	}, nil
}

// GetEntryByID returns a unmarshalled entry of the db by a given ID
func (store *Store) GetEntryByID(id string) (*shared.Entry, error) {
	if id == "" {
		return nil, shared.ErrNoEntryFound
	}
	return store.storage.GetEntryByID(id)
}

// GetEntryAndIncrease Increases the visitor count, checks
// if the URL is expired and returns the origin URL
func (store *Store) GetEntryAndIncrease(id string) (*shared.Entry, error) {
	entry, err := store.GetEntryByID(id)
	if err != nil {
		return nil, errors.Wrap(err, "could not fetch entry "+id)
	}

	if entry.Public.Expiration != nil && !entry.Public.Expiration.IsZero() && time.Now().Unix() > entry.Public.Expiration.Unix() {
		return nil, ErrEntryIsExpired
	}

	if err := store.storage.IncreaseVisitCounter(id); err != nil {
		return nil, errors.Wrap(err, "could not increase visitor counter")
	}

	entry.Public.VisitCount++
	return entry, nil
}

// CreateEntry creates a new record and returns his short id
func (store *Store) CreateEntry(entry shared.Entry, givenID, password string) (string, []byte, error) {
	entry.Public.URL = strings.Replace(entry.Public.URL, " ", "%20", -1)
	if !govalidator.IsURL(entry.Public.URL) {
		return "", nil, ErrNoValidURL
	}

	if password != "" {
		var err error
		entry.Password, err = bcrypt.GenerateFromPassword([]byte(password), 10)
		if err != nil {
			return "", nil, errors.Wrap(err, "could not generate bcrypt from password")
		}
	}

	// try it 10 times to make a short URL
	for i := 1; i <= 10; i++ {
		id, passwordHash, err := store.createEntry(entry, givenID)
		if err != nil && givenID != "" {
			return "", nil, err
		} else if err != nil {
			logger.Debugf("Could not create entry: %v", err)
			continue
		}

		return id, passwordHash, nil
	}

	return "", nil, ErrGeneratingIDFailed
}

// DeleteEntry deletes an Entry fully from the DB
func (store *Store) DeleteEntry(id string, givenHmac []byte) error {
	mac := hmac.New(sha512.New, util.GetPrivateKey())
	if _, err := mac.Write([]byte(id)); err != nil {
		return errors.Wrap(err, "could not write hmac")
	}

	if !hmac.Equal(mac.Sum(nil), givenHmac) {
		return errors.New("hmac verification failed")
	}

	return errors.Wrap(store.storage.DeleteEntry(id), "could not delete entry")
}

// RegisterVisit registers an new incoming request in the store
func (store *Store) RegisterVisit(id string, visitor shared.Visitor) {
	requestID := uuid.New()
	logger.Infof("[%s][%s][%s]New redirect was registered...", requestID, id, visitor.IP)

	if err := store.storage.RegisterVisitor(id, requestID, visitor); err != nil {
		logger.Warnf("could not register visit: %v", err)
	}
}

// GetVisitors returns all the visits of a shorted URL
func (store *Store) GetVisitors(id string) ([]shared.Visitor, error) {
	visitors, err := store.storage.GetVisitors(id)
	if err != nil {
		return nil, errors.Wrap(err, "could not get visitors")
	}

	return visitors, nil
}

func (store *Store) GetEntries() (map[string]shared.Entry, error) {
	entries, err := store.storage.GetEntries()
	if err != nil {
		return nil, errors.Wrap(err, "could not get entries")
	}

	return entries, nil
}

// Close closes the bolt db database
func (store *Store) Close() error {
	return store.storage.Close()
}

// createEntry creates a new entry with a randomly generated id. If on is present
// then the given ID is used
func (store *Store) createEntry(entry shared.Entry, entryID string) (string, []byte, error) {
	var err error
	if entryID == "" {
		if entryID, err = generateRandomString(store.idLength); err != nil {
			return "", nil, errors.Wrap(err, "could not generate random string")
		}
	}

	entry.Public.CreatedOn = &shared.Datetime{Time: time.Now()}
	mac := hmac.New(sha512.New, util.GetPrivateKey())

	if _, err := mac.Write([]byte(entryID)); err != nil {
		return "", nil, errors.Wrap(err, "could not write hmac")
	}

	if err := store.storage.CreateEntry(entry, entryID); err != nil {
		return "", nil, errors.Wrap(err, "could not create entry")
	}

	return entryID, mac.Sum(nil), nil
}

// generateRandomString generates a random string with an predefined length
func generateRandomString(length int) (string, error) {
	var result string
	for len(result) < length {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(127)))
		if err != nil {
			return "", err
		}

		n := num.Int64()
		if unicode.IsLetter(rune(n)) {
			result += string(n)
		}
	}

	return result, nil
}
