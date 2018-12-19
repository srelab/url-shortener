package handlers

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/srelab/url-shortener/pkg/stores/shared"
	"github.com/srelab/url-shortener/pkg/util"

	"github.com/labstack/echo"
)

type UrlHandler struct {
	*Handler
}

func (handler UrlHandler) Init() {
	group := handler.engine.Group("/api/v1/urls")
	group.POST("/", handler.create)
	group.GET("/recent", handler.recent)
	group.GET("/:id/lookup", handler.lookup)
	group.DELETE("/:id/:hash", handler.delete)
}

func (handler *Handler) create(ctx echo.Context) error {
	payload := new(URLPayLoad)
	if err := ctx.Bind(payload); err != nil {
		return FailureResponse(ctx, http.StatusBadRequest, ApiErrorParameter, err)
	}

	if payload.Expiration == nil {
		payload.Expiration = &Datetime{}
	}

	id, delID, err := handler.store.CreateEntry(shared.Entry{
		Public:     shared.EntryPublicData{URL: payload.URL, Expiration: &payload.Expiration.Time},
		RemoteAddr: ctx.RealIP(),
	}, payload.ID, payload.Password)

	if err != nil {
		if strings.Contains(err.Error(), shared.ErrEntryAlreadyExist.Error()) {
			return FailureResponse(ctx, http.StatusBadRequest, ApiErrorResourceAlreadyExists, err)
		}

		return FailureResponse(ctx, http.StatusInternalServerError, ApiErrorSystem, err)
	}

	payload.ID = id
	payload.URL = fmt.Sprintf("%s/%s", handler.GetURLOrigin(ctx), id)
	payload.DeletionURL = fmt.Sprintf(
		"%s/%s/%s", handler.GetURLOrigin(ctx), id, url.QueryEscape(base64.RawURLEncoding.EncodeToString(delID)),
	)
	
	return SuccessResponse(ctx, http.StatusOK, &HandlerResult{Result: payload})
}

func (handler *Handler) recent(ctx echo.Context) error {
	entries, err := handler.store.GetEntries()
	if err != nil {
		return FailureResponse(ctx, http.StatusNotFound, ApiErrorSystem, err)
	}

	for k, entry := range entries {
		mac := hmac.New(sha512.New, util.GetPrivateKey())
		if _, err := mac.Write([]byte(k)); err != nil {
			return FailureResponse(ctx, http.StatusNotFound, ApiErrorResourceNotExists, err)
		}

		entry.DeletionURL = fmt.Sprintf(
			"%s/d/%s/%s",
			handler.GetURLOrigin(ctx),
			k,
			url.QueryEscape(base64.RawURLEncoding.EncodeToString(mac.Sum(nil))),
		)

		entries[k] = entry
	}

	return SuccessResponse(ctx, http.StatusOK, &HandlerResult{
		Result: entries,
	})
}

func (handler *Handler) lookup(ctx echo.Context) error {
	id := ctx.Param("id")
	entry, err := handler.store.GetEntryByID(id)

	if err != nil {
		return FailureResponse(ctx, http.StatusNotFound, ApiErrorResourceNotExists, err)
	}

	return SuccessResponse(ctx, http.StatusOK, &HandlerResult{
		Result: shared.Entry{
			Public: shared.EntryPublicData{URL: entry.Public.URL},
		},
	})
}

func (handler *Handler) delete(ctx echo.Context) error {
	givenHmac, err := base64.RawURLEncoding.DecodeString(ctx.Param("hash"))
	if err != nil {
		return FailureResponse(ctx, http.StatusInternalServerError, ApiErrorSystem, err)
	}

	if err := handler.store.DeleteEntry(ctx.Param("id"), givenHmac); err != nil {
		return FailureResponse(ctx, http.StatusNotFound, ApiErrorResourceNotExists, err)
	}

	return SuccessResponse(ctx, http.StatusOK, &HandlerResult{})
}
