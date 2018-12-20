package shared

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/srelab/url-shortener/pkg/g"
)

const defaultExpiration = time.Second * 60

type Datetime struct {
	time.Time
}

func (d *Datetime) UnmarshalJSON(b []byte) (err error) {
	str := strings.Trim(string(b), "\"")
	if str == "null" || str == "" {
		d.Time = time.Time{}
		return
	}

	d.Time, err = time.ParseInLocation(g.DefaultTimeFormat, str, time.Local)
	return
}

func (d *Datetime) MarshalJSON() ([]byte, error) {
	if d.Time.UnixNano() == (time.Time{}).UnixNano() {
		return []byte("null"), nil
	}

	return []byte(fmt.Sprintf("\"%s\"", d.Time.Format(g.DefaultTimeFormat))), nil
}

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
	RemoteAddr  string          `json:"remote_addr,omitempty"`
	DeletionURL string          `json:"deletion_url,omitempty"`
	Password    []byte          `json:"password,omitempty"`
	Public      EntryPublicData `json:"public"`
}

// GetExpiration calculate the difference by expiration time
func (entry *Entry) GetExpiration() time.Duration {
	if entry.Public.Expiration == nil || entry.Public.Expiration.IsZero() {
		return 0
	}

	expiration := time.Duration(entry.Public.Expiration.Time.UnixNano() - time.Now().UnixNano())
	if expiration < defaultExpiration {
		// default expiration time (duration)
		expiration = defaultExpiration
	}

	return expiration
}

// EntryPublicData is the public part of an entry
type EntryPublicData struct {
	CreatedOn  *Datetime `json:"created_on"`
	LastVisit  *Datetime `json:"last_visit,omitempty"`
	Expiration *Datetime `json:"expiration,omitempty"`
	VisitCount int       `json:"visit_count"`
	URL        string    `json:"url"`
}

// Visitor is the entry which is stored in the visitors bucket
type Visitor struct {
	IP          string    `json:"ip"`
	Referer     string    `json:"referer"`
	UserAgent   string    `json:"user_agent"`
	Timestamp   *Datetime `json:"timestamp"`
	UTMSource   string    `json:"utm_source,omitempty"`
	UTMMedium   string    `json:"utm_medium,omitempty"`
	UTMCampaign string    `json:"utm_campaign,omitempty"`
	UTMContent  string    `json:"utm_content,omitempty"`
	UTMTerm     string    `json:"utm_term,omitempty"`

	Expiration time.Duration `json:"-"`
}

// ErrNoEntryFound is returned when no entry to a id is found
var ErrNoEntryFound = errors.New("no entry found with this ID")
var ErrEntryAlreadyExist = errors.New("already exists")
