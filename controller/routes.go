package controller

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/spf13/viper"

	"github.com/bacchus-snu/sgs/model"
	"github.com/bacchus-snu/sgs/pkg/auth"
	"github.com/bacchus-snu/sgs/view"
	"github.com/bacchus-snu/sgs/worker"
)

type Config struct {
	SessionKey string `mapstructure:"session_key"`
	sessionKey []byte
}

func (c *Config) Bind() {
	viper.BindEnv("controller.session_key", "SGS_SESSION_KEY")
}

func (c *Config) Validate() error {
	if c.SessionKey == "" {
		return errors.New("session_key is required")
	}

	d, err := hex.DecodeString(c.SessionKey)
	if err != nil {
		return fmt.Errorf("invalid session_key: %w", err)
	}
	c.sessionKey = d

	return nil
}

func AddRoutes(
	e *echo.Echo,
	cfg Config,
	queue worker.Queue,
	authSvc auth.Service,
	wsSvc model.WorkspaceService,
) {
	stor := sessions.NewCookieStore(cfg.sessionKey)
	stor.Options.SameSite = http.SameSiteLaxMode
	stor.Options.Secure = true
	stor.Options.HttpOnly = true

	e.Renderer = view.Renderer
	e.HTTPErrorHandler = view.ErrorHandler

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	e.Use(
		// generic
		middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
			LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
				if v.Error == nil {
					logger.LogAttrs(context.Background(), slog.LevelInfo, "REQUEST",
						slog.String("uri", v.URI),
						slog.Int("status", v.Status),
					)
				} else {
					logger.LogAttrs(context.Background(), slog.LevelError, "REQUEST_ERROR",
						slog.String("uri", v.URI),
						slog.Int("status", v.Status),
						slog.String("err", v.Error.Error()),
					)
				}
				return nil
			},

			//HandleError:     true,
			LogLatency:      true,
			LogRemoteIP:     true,
			LogMethod:       true,
			LogURI:          true,
			LogUserAgent:    true,
			LogStatus:       true,
			LogError:        true,
			LogResponseSize: true,
		}),
		middleware.Recover(),

		// csrf
		middleware.CSRFWithConfig(middleware.CSRFConfig{
			TokenLookup:    "form:_csrf",
			ContextKey:     "csrf",
			CookieSecure:   true,
			CookieHTTPOnly: true,
			CookieSameSite: http.SameSiteLaxMode,
		}),

		middleware.Secure(),
		middleware.Gzip(),

		session.Middleware(stor),
		middlewareAuth(),
	)

	e.GET("/healthz", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	e.StaticFS("/static", view.Static)

	e.GET("/auth", handleAuth(authSvc)).Name = "auth"
	e.GET("/auth/callback", handleAuthCallback(authSvc))
	e.GET("/auth/logout", handleAuthLogout())

	requireAuth := middlewareAuthenticated()

	e.GET("/", handleListWorkspaces(wsSvc), requireAuth).Name = "workspace-list"
	e.GET("/ws/:id", handleWorkspaceDetails(wsSvc), requireAuth).Name = "workspace-details"
	e.POST("/ws/:id", handleUpdateWorkspace(queue, wsSvc), requireAuth)

	e.GET("/request", handleRequestWorkspaceForm(), requireAuth)
	e.POST("/request", handleRequestWorkspace(wsSvc), requireAuth)
}
