package handlers

import (
	"net/http"
	"runtime"
	"strings"

	"github.com/srelab/url-shortener/pkg/g"

	"github.com/labstack/echo"
)

type PublicHandler struct {
	*Handler
}

func (handler PublicHandler) Init() {
	group := handler.engine.Group("/api/v1/publics")
	group.GET("/", handler.get)
	group.GET("/info", handler.info)
	group.GET("/health", handler.info)
}

func (PublicHandler) get(ctx echo.Context) error {
	return SuccessResponse(ctx, http.StatusOK, nil)
}

func (handler *Handler) info(ctx echo.Context) error {
	return SuccessResponse(ctx, http.StatusOK, &HandlerResult{
		Result: map[string]string{
			"go":      strings.Replace(runtime.Version(), "go", "", 1),
			"version": g.VERSION,
		},
	})
}

func (handler *Handler) health(ctx echo.Context) error {
	return SuccessResponse(ctx, http.StatusOK, &HandlerResult{
		Result: map[string]string{
			"status": "ok",
		},
	})
}
