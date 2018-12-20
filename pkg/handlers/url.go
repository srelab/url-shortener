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

const prefix = "/api/v1/urls"

type UrlHandler struct {
	*Handler
}

func (handler UrlHandler) Init() {
	group := handler.engine.Group(prefix)
	group.GET("", handler.all)
	group.POST("", handler.create)

	group.GET("/:id/lookup", handler.lookup)
	group.GET("/:id/visitors", handler.visitors)
	group.DELETE("/:id/:hash", handler.delete)
}

func (handler *Handler) create(ctx echo.Context) error {
	payload := new(URLPayLoad)
	if err := ctx.Bind(payload); err != nil {
		return FailureResponse(ctx, http.StatusBadRequest, ApiErrorParameter, err)
	}

	id, delID, err := handler.store.CreateEntry(shared.Entry{
		Public:     shared.EntryPublicData{URL: payload.URL, Expiration: payload.Expiration},
		RemoteAddr: ctx.RealIP(),
	}, payload.ID, payload.Password)

	if err != nil {
		if strings.Contains(err.Error(), shared.ErrEntryAlreadyExist.Error()) {
			return FailureResponse(ctx, http.StatusBadRequest, ApiErrorResourceAlreadyExists, err)
		}

		return FailureResponse(ctx, http.StatusInternalServerError, ApiErrorSystem, err)
	}

	payload.ID = id
	payload.URL = fmt.Sprintf("%s/%s", handler.getURL(ctx), id)
	payload.DeletionURL = fmt.Sprintf(
		"%s/%s/%s", handler.getDeletionURL(ctx, prefix), id, url.QueryEscape(base64.RawURLEncoding.EncodeToString(delID)),
	)

	return SuccessResponse(ctx, http.StatusOK, &HandlerResult{Result: payload})
}

func (handler *Handler) all(ctx echo.Context) error {
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
			"%s/%s/%s",
			handler.getDeletionURL(ctx, prefix),
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
		Result: entry,
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

func (handler *Handler) visitors(ctx echo.Context) error {
	id := ctx.Param("id")
	visitors, err := handler.store.GetVisitors(id)

	if err != nil {
		return FailureResponse(ctx, http.StatusNotFound, ApiErrorResourceNotExists, err)
	}

	return SuccessResponse(ctx, http.StatusOK, &HandlerResult{
		Result: visitors,
	})
}
