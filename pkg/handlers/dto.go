package handlers

import (
	"fmt"
	"strings"
	"time"

	"github.com/srelab/url-shortener/pkg/g"
)

type URLPayLoad struct {
	ID          string    `json:"id"                validate:"-"`
	URL         string    `json:"url"               validate:"required,url"`
	DeletionURL string    `json:"deletion_url"      validate:"-"`
	Password    string    `json:"password"          validate:"-"`
	Expiration  *Datetime `json:"expiration"          validate:"-"`
}

type PasswordPayLoad struct {
	Password string `json:"password" validate:"required"`
}

type Datetime struct {
	time.Time
}

func (d *Datetime) UnmarshalJSON(b []byte) (err error) {
	str := strings.Trim(string(b), "\"")
	fmt.Println(str)
	if str == "null" {
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
