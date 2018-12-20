package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/go-playground/validator"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"

	"github.com/srelab/url-shortener/pkg/g"
	"github.com/srelab/url-shortener/pkg/logger"
	"github.com/srelab/url-shortener/pkg/stores"
	"github.com/srelab/url-shortener/pkg/stores/shared"
)

type HandlerResult struct {
	Result     interface{}  `json:"result"`
	Success    bool         `json:"success"`
	Error      HandlerError `json:"error,omitempty"`
	Pagination interface{}  `json:"pagination,omitempty"`
}

type HandlerError struct {
	Code    int         `json:"code,omitempty"`
	Message string      `json:"msg,omitempty"`
	Details interface{} `json:"details,omitempty"`
}

var (
	ApiErrorSystem             = HandlerError{Code: 1001, Message: "System Error"}
	ApiErrorServiceUnavailable = HandlerError{Code: 1002, Message: "Service unavailable"}
	ApiErrorNotFound           = HandlerError{Code: 1003, Message: "Resource not found"}
	ApiErrorHTTPMethod         = HandlerError{Code: 1004, Message: "HTTP method is not suported for this request"}

	ApiErrorParameter             = HandlerError{Code: 1101, Message: "Parameter error"}
	ApiErrorResourceNotExists     = HandlerError{Code: 1102, Message: "Resource does not exists"}
	ApiErrorResourceAlreadyExists = HandlerError{Code: 1103, Message: "Resource already exists"}
	ApiErrorPasswordInvalid       = HandlerError{Code: 1104, Message: "Password invalid"}
)

func FailureResponse(ctx echo.Context, status int, he HandlerError, err error, v ...interface{}) error {
	str := ""
	if err != nil {
		str = err.Error()
	}
	return ctx.JSON(status, HandlerResult{
		Success: false,
		Error: HandlerError{
			Code:    he.Code,
			Message: fmt.Sprintf(he.Message, v...),
			Details: str,
		},
	})
}

func SuccessResponse(ctx echo.Context, status int, hr *HandlerResult) error {
	if hr == nil {
		hr = new(HandlerResult)
	}

	if hr.Result == nil || hr.Result == "" {
		hr.Result = "request success"
	}

	hr.Success = true
	return ctx.JSON(status, hr)
}

type HandlerValidator struct {
	validate *validator.Validate
}

func (hv *HandlerValidator) Validate(i interface{}) error {
	return hv.validate.Struct(i)
}

type BinderWithValidation struct{}

func (BinderWithValidation) Bind(i interface{}, ctx echo.Context) error {
	binder := &echo.DefaultBinder{}

	if err := binder.Bind(i, ctx); err != nil {
		return errors.New(err.(*echo.HTTPError).Message.(string))
	}

	if err := ctx.Validate(i); err != nil {
		var buf bytes.Buffer

		for _, fieldErr := range err.(validator.ValidationErrors) {
			buf.WriteString("Validation failed on ")
			buf.WriteString(fieldErr.Tag())
			buf.WriteString(" for ")
			buf.WriteString(fieldErr.StructField())
			buf.WriteString("\n")
		}

		return errors.New(buf.String())
	}

	return nil
}

type Handler struct {
	store  stores.Store
	engine *echo.Echo
}

func (handler *Handler) GetURLOrigin(ctx echo.Context) string {
	protocol := "http"
	if ctx.Request().TLS != nil || ctx.Request().Header.Get("X-Forwarded-Proto") == "https" {
		protocol = "https"
	}

	location := strings.Trim(g.GetConfig().Location, "/")
	if location != "" {
		return fmt.Sprintf("%s://%s/%s", protocol, ctx.Request().Host, location)
	}

	return fmt.Sprintf("%s://%s", protocol, ctx.Request().Host)
}

// New initializes the http handlers
func New(store stores.Store) (*Handler, error) {
	handler := &Handler{
		store:  store,
		engine: echo.New(),
	}

	handler.engine.HideBanner = true
	if logger.GetLogLevel() == logger.LevelDebug {
		handler.engine.Debug = true
	}

	handler.engine.Use(middleware.CORS())
	handler.engine.Use(middleware.Recover())
	handler.engine.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Skipper: middleware.DefaultSkipper,
		Format:  middleware.DefaultLoggerConfig.Format,
		Output:  logger.GetLogWriter("access.log"),
	}))

	handler.engine.Binder = &BinderWithValidation{}
	handler.engine.Validator = func() echo.Validator {
		v := validator.New()

		_ = v.RegisterValidation("json", func(fl validator.FieldLevel) bool {
			var js json.RawMessage
			return json.Unmarshal([]byte(fl.Field().String()), &js) == nil
		})

		_ = v.RegisterValidation("in", func(fl validator.FieldLevel) bool {
			values := strings.Split(fl.Param(), ";")
			fieldValue := fmt.Sprintf("%v", fl.Field())

			for _, value := range values {
				if value == fieldValue {
					return true
				}
			}

			return false
		})

		return &HandlerValidator{validate: v}

	}()

	handler.engine.HTTPErrorHandler = func(err error, ctx echo.Context) {
		var (
			code = http.StatusInternalServerError
		)

		if httpError, ok := err.(*echo.HTTPError); ok {
			code = httpError.Code
		}

		if !ctx.Response().Committed {
			// https://www.w3.org/Protocols/rfc2616/rfc2616-sec9.html
			if ctx.Request().Method == echo.HEAD {
				if err := ctx.NoContent(code); err != nil {
					goto ERROR
				}
			}

			switch code {
			case http.StatusNotFound:
				if err := FailureResponse(ctx, code, ApiErrorNotFound, nil); err != nil {
					goto ERROR
				}
			case http.StatusMethodNotAllowed:
				if err := FailureResponse(ctx, code, ApiErrorHTTPMethod, nil); err != nil {
					goto ERROR
				}
			case http.StatusInternalServerError:
				if err := FailureResponse(ctx, code, ApiErrorSystem, nil); err != nil {
					goto ERROR
				}
			default:
				if err := FailureResponse(ctx, code, ApiErrorServiceUnavailable, nil); err != nil {
					goto ERROR
				}
			}
		}

	ERROR:
		logger.Error(err)

	}

	PublicHandler{Handler: handler}.Init()
	UrlHandler{Handler: handler}.Init()

	handler.engine.GET("*", func(ctx echo.Context) error {
		id := ctx.Request().URL.Path[1:]
		entry, err := handler.store.GetEntryAndIncrease(id)
		if err != nil {
			if strings.Contains(err.Error(), shared.ErrNoEntryFound.Error()) {
				return FailureResponse(ctx, http.StatusNotFound, ApiErrorResourceNotExists, err)
			}

			return FailureResponse(ctx, http.StatusInternalServerError, ApiErrorResourceNotExists, err)
		}

		if len(entry.Password) == 0 {
			go handler.RegisterVisitor(id, ctx)
			return ctx.Redirect(http.StatusTemporaryRedirect, entry.Public.URL)
		}

		payload := new(PasswordPayLoad)
		if err := ctx.Bind(payload); err != nil {
			return FailureResponse(ctx, http.StatusBadRequest, ApiErrorParameter, err)
		}

		if err := bcrypt.CompareHashAndPassword(entry.Password, []byte(payload.Password)); err != nil {
			return FailureResponse(ctx, http.StatusBadRequest, ApiErrorPasswordInvalid, err)
		}

		go handler.RegisterVisitor(id, ctx)
		return ctx.Redirect(http.StatusTemporaryRedirect, entry.Public.URL)
	})

	return handler, nil
}

func (handler *Handler) RegisterVisitor(id string, ctx echo.Context) {
	handler.store.RegisterVisit(id, shared.Visitor{
		IP:          ctx.RealIP(),
		Timestamp:   &shared.Datetime{Time: time.Now()},
		Referer:     ctx.Request().Header.Get("Referer"),
		UserAgent:   ctx.Request().Header.Get("User-Agent"),
		UTMSource:   ctx.QueryParam("utm_source"),
		UTMMedium:   ctx.QueryParam("utm_medium"),
		UTMCampaign: ctx.QueryParam("utm_campaign"),
		UTMContent:  ctx.QueryParam("utm_content"),
		UTMTerm:     ctx.QueryParam("utm_term"),
	})
}

// Listen starts the http server
func (handler *Handler) Listen() error {
	return handler.engine.Start(g.GetConfig().ListenAddr)
}

// CloseStore stops the http server and the closes the db gracefully
func (handler *Handler) CloseStore() error {
	return handler.store.Close()
}
