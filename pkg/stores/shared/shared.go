package shared

import (
	"errors"
	"time"
)

// Storage is an interface which will be implmented by each storage
// e.g. bolt, sqlite
type Storage interface {
	GetEntryByID(string) (*Entry, error)
	GetVisitors(string) ([]Visitor, error)
	DeleteEntry(string) error
	IncreaseVisitCounter(string) error
	CreateEntry(Entry, string) error
	GetEntries() (map[string]Entry, error)
	RegisterVisitor(string, string, Visitor) error
	Close() error
}

// Entry is the data set which is stored in the DB as JSON
type Entry struct {
	RemoteAddr  string `json:",omitempty"`
	DeletionURL string `json:",omitempty"`
	Password    []byte `json:",omitempty"`
	Public      EntryPublicData
}

// EntryPublicData is the public part of an entry
type EntryPublicData struct {
	CreatedOn  time.Time
	LastVisit  *time.Time `json:",omitempty"`
	Expiration *time.Time `json:",omitempty"`
	VisitCount int
	URL        string
}

// Visitor is the entry which is stored in the visitors bucket
type Visitor struct {
	IP, Referer, UserAgent string
	Timestamp              time.Time
	UTMSource              string `json:",omitempty"`
	UTMMedium              string `json:",omitempty"`
	UTMCampaign            string `json:",omitempty"`
	UTMContent             string `json:",omitempty"`
	UTMTerm                string `json:",omitempty"`
}

// ErrNoEntryFound is returned when no entry to a id is found
var ErrNoEntryFound = errors.New("no entry found with this ID")
var ErrEntryAlreadyExist = errors.New("already exists")
