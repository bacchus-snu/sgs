package controller

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/bacchus-snu/sgs/model"
	"github.com/bacchus-snu/sgs/pkg/auth"
)

func handleSubscribe(mlSvc model.MailingListService) echo.HandlerFunc {
	return func(c echo.Context) error {
		user := c.Get("user").(*auth.User)
		if !user.IsAdmin() {
			return echo.ErrForbidden
		}

		if err := mlSvc.Subscribe(c.Request().Context(), user.Username, user.Email); err != nil {
			return err
		}

		return c.Redirect(http.StatusSeeOther, c.Echo().Reverse("workspace-list"))
	}
}

func handleUnsubscribe(mlSvc model.MailingListService) echo.HandlerFunc {
	return func(c echo.Context) error {
		user := c.Get("user").(*auth.User)
		if !user.IsAdmin() {
			return echo.ErrForbidden
		}

		if err := mlSvc.Unsubscribe(c.Request().Context(), user.Username); err != nil {
			return err
		}

		return c.Redirect(http.StatusSeeOther, c.Echo().Reverse("workspace-list"))
	}
}
