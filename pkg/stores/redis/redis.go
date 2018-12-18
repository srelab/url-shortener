package redis

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis"
	"github.com/pkg/errors"
	"github.com/srelab/url-shortener/pkg/logger"
	"github.com/srelab/url-shortener/pkg/stores/shared"
)

var (
	entryKeyPrefix       = "entry:"        // prefix for path-to-url mappings
	entryVisitsKeyPrefix = "entry:visits:" // prefix for entry-to-[]visit mappings (redis LIST)

	entriesKey = "entries" // prefix for []entries mappings (redis SET)
)

// Store implements the stores.Storage interface
type Storage struct {
	client *redis.Client
}

// New initializes connection to the redis instance.
func New(hostaddr, password string, db int, maxRetries int, readTimeout string, writeTimeout string) (*Storage, error) {
	var rt, wt time.Duration
	var err error

	if rt, err = time.ParseDuration(readTimeout); err != nil {
		return nil, errors.Wrap(err, "Could not parse read timeout")
	}

	if wt, err = time.ParseDuration(writeTimeout); err != nil {
		return nil, errors.Wrap(err, "Could not parse write timeout")
	}

	client := redis.NewClient(&redis.Options{
		Addr:         hostaddr,
		Password:     password,
		DB:           db,
		MaxRetries:   maxRetries,
		ReadTimeout:  rt,
		WriteTimeout: wt,
	})

	// if we can't talk to redis, fail fast
	if _, err = client.Ping().Result(); err != nil {
		return nil, errors.Wrap(err, "Could not connect to redis db0")
	}

	result := &Storage{client: client}
	return result, nil
}

// keyExists checks for the existence of a key in redis.
func (storage *Storage) keyExists(key string) (exists bool, err error) {
	logger.Debugf("Checking for existence of key: %s", key)
	result := storage.client.Exists(key)

	if result.Err() != nil {
		errmsg := fmt.Sprintf("Error looking up key '%s': '%v', got val: '%d'", key, result.Err(), result.Val())

		logger.Error(errmsg)
		return false, errors.Wrap(result.Err(), errmsg)
	}

	if result.Val() == 1 {
		logger.Debugf("Key '%s' exists!", key)
		return true, nil
	}

	logger.Debugf("Key '%s' does not exist!", key)
	return false, nil
}

// setValue sets the value of a key in redis.
func (storage *Storage) setValue(key string, raw []byte) error {
	logger.Debugf("Setting value for key '%s: '%s''", key, raw)
	// n.b. expiration 0 means never expire
	status := storage.client.Set(key, raw, 0)

	if status.Err() != nil {
		errmsg := fmt.Sprintf("Got an unexpected error adding key '%s': %s", key, status.Err())

		logger.Error(errmsg)
		return errors.Wrap(status.Err(), errmsg)
	}

	return nil
}

// createValue is a wrapper around setValue that returns an error if the key already exists.
func (storage *Storage) createValue(key string, raw []byte) error {
	logger.Debugf("Creating key '%s'", key)

	exists, err := storage.keyExists(key)
	if err != nil {
		errmsg := fmt.Sprintf("Could not check existence of key '%s': %s", key, err)

		logger.Error(errmsg)
		return errors.Wrap(err, errmsg)
	}

	if exists == true {
		errmsg := fmt.Sprintf("Could not create key '%s': already exists", key)

		logger.Error(errmsg)
		return errors.New(errmsg)
	}

	return storage.setValue(key, raw)
}

// delValue deletes a key in redis.
func (storage *Storage) delValue(key string) error {
	logger.Debugf("Deleting key '%s'", key)

	exists, err := storage.keyExists(key)
	if err != nil {
		errmsg := fmt.Sprintf("Could not check existence of key '%s': %s", key, err)

		logger.Error(errmsg)
		return errors.Wrap(err, errmsg)
	}

	if exists == false {
		errmsg := fmt.Sprintf("Tried to delete key '%s' but it's already gone", key)

		logger.Warnf(errmsg)
		return err
	}

	status := storage.client.Del(key)
	if status.Err() != nil {
		errmsg := fmt.Sprintf("Got an unexpected error deleting key '%s': %s", key, status.Err())

		logger.Error(errmsg)
		return errors.Wrap(status.Err(), errmsg)
	}

	return err
}

// CreateEntry creates an entry (path->url mapping) and all associated stored data.
func (storage *Storage) CreateEntry(entry shared.Entry, id string) error {
	// add the entry (path->url mapping)
	logger.Debugf("Creating entry '%s' for user '%s'", id)

	raw, err := json.Marshal(entry)
	if err != nil {
		errmsg := fmt.Sprintf("Could not marshal JSON for entry %s: %v", id, err)

		logger.Error(errmsg)
		return errors.Wrap(err, errmsg)
	}

	entryKey := entryKeyPrefix + id
	logger.Debugf("Adding key '%s': %s", entryKey, raw)

	err = storage.createValue(entryKey, raw)
	if err != nil {
		errmsg := fmt.Sprintf("Failed to set key '%s': %v", entryKey, err)

		logger.Error(errmsg)
		return errors.Wrap(err, errmsg)
	}

	// add the entry to the SET of entries
	logger.Debugf("Adding entry '%s' to set of entries '%s'", id)
	result := storage.client.SAdd(entriesKey, id)
	if result.Err() != nil {
		errmsg := fmt.Sprintf("Failed to add entry '%s': %v", id, result.Err())

		logger.Error(errmsg)
		return errors.Wrap(result.Err(), errmsg)
	}

	logger.Debugf("Successfully added entry '%s' to set '%s'", id, entriesKey)
	return nil
}

// DeleteEntry deletes an entry and all associated stored data.
func (storage *Storage) DeleteEntry(id string) error {
	// delete the id-to-url mapping
	entryKey := entryKeyPrefix + id
	err := storage.delValue(entryKey)
	if err != nil {
		errmsg := fmt.Sprintf("Could not delete entry id %s: %v", id, err)

		logger.Error(errmsg)
		return errors.Wrap(err, errmsg)
	}

	// delete the visitors list for the id
	entryVisitsKey := entryVisitsKeyPrefix + id
	err = storage.delValue(entryVisitsKey)
	if err != nil {
		errmsg := fmt.Sprintf("Could not delete visitors list for id %s: %v", id, err)

		logger.Error(errmsg)
		return errors.Wrap(err, errmsg)
	}

	// delete the entry from set of entries for the user
	err = storage.client.SRem(entriesKey+entryKey, id).Err()
	if err != nil {
		errmsg := fmt.Sprintf("Could not remove entry '%s' from list of entries: %v", id, err)

		logger.Error(errmsg)
		return errors.Wrap(err, errmsg)
	}

	// delete the id-to-user mapping
	err = storage.delValue(entryKey)
	if err != nil {
		errmsg := fmt.Sprintf("Could not delete the path-to-user mapping for entry '%s': %v", id, err)

		logger.Error(errmsg)
		return errors.Wrap(err, errmsg)
	}

	return err
}

// GetEntryByID looks up an entry by its path and returns a pointer to a
// shared.Entry instance, with the visit count and last visit time set
// properly.
func (storage *Storage) GetEntryByID(id string) (*shared.Entry, error) {
	entryKey := entryKeyPrefix + id
	logger.Debugf("Fetching key: '%s'", entryKey)

	result := storage.client.Get(entryKey)
	raw, err := result.Bytes()
	if err != nil {
		msg := fmt.Sprintf("Error looking up key '%s': %s'", entryKey, err)
		logger.Warn(msg)

		err = shared.ErrNoEntryFound
		return nil, err
	}

	logger.Debugf("Got entry for key '%s': '%s'", entryKey, raw)

	var entry *shared.Entry
	err = json.Unmarshal(raw, &entry)
	if err != nil {
		errmsg := fmt.Sprintf("Error unmarshalling JSON for entry '%s': %v  (json str: '%s')", id, err, raw)

		logger.Error(errmsg)
		return nil, errors.Wrap(err, errmsg)
	}

	// now we interleave the visit count and the last visit time
	// from the redis sources (we do this so we don't have to rewrite
	// the entry every time someone visits which is madness)
	//
	// first, the visit count is just the length of the visitors list
	entryVisitsKey := entryVisitsKeyPrefix + id
	visitCount, err := storage.client.LLen(entryVisitsKey).Result()
	if err != nil {
		logger.Warnf("Could not get length of visitor list for id '%s': '%v'", id, err)
		entry.Public.VisitCount = int(0) // or zero if nobody's visited, that's fine.
	} else {
		entry.Public.VisitCount = int(visitCount)
	}

	// grab the timestamp out of the last visitor on the list
	var visitor *shared.Visitor
	lastVisit := time.Time(time.Unix(0, 0)) // default to start-of-epoch if we can't figure it out
	raw, err = storage.client.LIndex(entryVisitsKey, 0).Bytes()
	if err != nil {
		logger.Warnf("Could not fetch visitor list for entry '%s': %v", id, err)
	} else {
		err = json.Unmarshal(raw, &visitor)
		if err != nil {
			logger.Warnf("Could not unmarshal JSON for last visitor to entry '%s': %v  (got string: '%s')", id, err, raw)
		} else {
			lastVisit = visitor.Timestamp
		}
	}

	logger.Debugf("Setting last visit time for entry '%s' to '%v'", id, lastVisit)
	entry.Public.LastVisit = &lastVisit

	return entry, nil
}

// GetEntries returns all entries, in the form of a map of path->shared.Entry
func (storage *Storage) GetEntries() (map[string]shared.Entry, error) {
	entries := map[string]shared.Entry{}

	result := storage.client.SMembers(entriesKey)
	if result.Err() != nil {
		errmsg := fmt.Sprintf("Could not fetch set of entries for entries prefix '%s': %v", entriesKey, result.Err())

		logger.Errorf(errmsg)
		return nil, errors.Wrap(result.Err(), errmsg)
	}

	for _, v := range result.Val() {
		logger.Debugf("got entry: %s", v)

		entry, err := storage.GetEntryByID(string(v))
		if err != nil {
			msg := fmt.Sprintf("Could not get entry '%s': %s", v, err)
			logger.Warn(msg)
		} else {
			entries[string(v)] = *entry
		}
	}

	logger.Debugf("all out of entries")
	return entries, nil
}

// RegisterVisitor adds a shared.Visitor to the list of visits for a path.
func (storage *Storage) RegisterVisitor(id, visitID string, visitor shared.Visitor) error {
	data, err := json.Marshal(visitor)
	if err != nil {
		errmsg := fmt.Sprintf("Could not marshal JSON for entry %s, visitID %s: %s", id, visitID, err)

		logger.Error(errmsg)
		return errors.Wrap(err, errmsg)
	}

	// push the visit data onto a redis list who's key is the url id
	result := storage.client.LPush(entryVisitsKeyPrefix+id, data)
	if result.Err() != nil {
		errmsg := fmt.Sprintf("Could not register visitor for ID %s: %s", id, result.Err())

		logger.Error(errmsg)
		return errors.Wrap(err, errmsg)
	}
	return err
}

// GetVisitors returns the full list of visitors for a path.
func (storage *Storage) GetVisitors(id string) ([]shared.Visitor, error) {
	var visitors []shared.Visitor

	// TODO: for non-trivial numbers of keys, this could start
	// to get hairy; should convert to a paginated Scan operation.
	result := storage.client.LRange(entryVisitsKeyPrefix+id, 0, -1)
	if result.Err() != nil {
		errmsg := fmt.Sprintf("Could not get visitors for id '%s': %s", id, result.Err())

		logger.Error(errmsg)
		return nil, errors.Wrap(result.Err(), errmsg)
	}

	for _, v := range result.Val() {
		var value shared.Visitor
		if err := json.Unmarshal([]byte(v), &value); err != nil {
			errmsg := fmt.Sprintf("Could not unmarshal json for visit '%s': %v", id, err)

			logger.Error(errmsg)
			return nil, errors.Wrap(result.Err(), errmsg)
		}
		visitors = append(visitors, value)
	}
	return visitors, nil
}

// IncreaseVisitCounter is a no-op and returns nil for all values.
//
// This function is unnecessary for the redis backend: we already
// have a redis LIST of visitors, and we can derive the visit count
// by calling redis.client.LLen(list) (which is a constant-time op)
// during GetEntryByID().  If we want the timestamp of the most recent
// visit we can pull the most recent visit off with redis.client.LIndex(0)
// (also constant-time) and reading the timetamp field.
func (storage *Storage) IncreaseVisitCounter(id string) error {
	return nil
}

// Close closes the connection to redis.
func (storage *Storage) Close() error {
	err := storage.client.Close()

	if err != nil {
		errmsg := fmt.Sprintf("Cloud not close the redis connection: %s", err)

		logger.Error(errmsg)
		return errors.Wrap(err, errmsg)
	}
	return err
}
