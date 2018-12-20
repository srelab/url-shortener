package handlers

import (
	"github.com/srelab/url-shortener/pkg/stores/shared"
)

type URLPayLoad struct {
	ID          string           `json:"id"                validate:"-"`
	URL         string           `json:"url"               validate:"required,url"`
	DeletionURL string           `json:"deletion_url"      validate:"-"`
	Password    string           `json:"password"          validate:"-"`
	Expiration  *shared.Datetime `json:"expiration"          validate:"-"`
}

type PasswordPayLoad struct {
	Password string `json:"password" validate:"required"`
}
