package controller

import (
	"net/http"
	"net/url"

	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"

	"github.com/bacchus-snu/sgs/pkg/auth"
	"github.com/bacchus-snu/sgs/view"
)

// middlewareAuth adds the user to the context if they are authenticated.
func middlewareAuth() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			sess, _ := session.Get("session", c)
			if user, ok := sess.Values["user"].(*auth.User); ok {
				c.Set("user", user)
			}
			return next(c)
		}
	}
}

// middlewareAuthenticated redirects to the auth route if the user is not authenticated.
func middlewareAuthenticated() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, _ := c.Get("user").(*auth.User)

			if user == nil || user.Username == "" {
				c.SetCookie(&http.Cookie{
					Name:     "return_to",
					Value:    c.Request().URL.String(),
					Path:     c.Echo().Reverse("auth"),
					Secure:   true,
					HttpOnly: true,
					SameSite: http.SameSiteNoneMode,
				})
				return c.Redirect(http.StatusSeeOther, c.Echo().Reverse("auth"))
			}

			return next(c)
		}
	}
}

func handleAuth(
	authSvc auth.Service,
) echo.HandlerFunc {
	return func(c echo.Context) error {
		user, _ := c.Get("user").(*auth.User)

		if user != nil && user.Username != "" {
			// user is already authenticated, return to previous page
			returnTo := "/"

			cook, _ := c.Cookie("return_to")
			if cook != nil {
				retURL, err := url.ParseRequestURI(cook.Value)
				if err == nil {
					returnTo = retURL.String()
				}
			}
			// clean up after ourselves
			c.SetCookie(&http.Cookie{
				Name:   "return_to",
				Path:   c.Echo().Reverse("auth"),
				MaxAge: -1,
			})

			return c.Redirect(http.StatusSeeOther, returnTo)
		}

		url, verifier := authSvc.AuthURL()
		// redirect to auth, save verifier in session
		sess, _ := session.Get("session", c)
		sess.Values["auth_verifier"] = verifier
		sess.Save(c.Request(), c.Response())
		return c.Redirect(http.StatusSeeOther, url)
	}
}

func handleAuthCallback(
	authSvc auth.Service,
) echo.HandlerFunc {
	return func(c echo.Context) error {
		sess, _ := session.Get("session", c)
		verifier := sess.Values["auth_verifier"]

		user, err := authSvc.Exchange(
			c.Request().Context(),
			c.QueryParam("code"),
			c.QueryParam("state"),
			verifier,
		)
		if err != nil {
			return err
		}

		sess.Values["user"] = user
		sess.Save(c.Request(), c.Response())
		return c.Redirect(http.StatusSeeOther, c.Echo().Reverse("auth"))
	}
}

func handleAuthLogout() echo.HandlerFunc {
	return func(c echo.Context) error {
		sess, _ := session.Get("session", c)
		delete(sess.Values, "user")
		sess.Save(c.Request(), c.Response())

		// Clear user from context so header doesn't show logged in state
		c.Set("user", nil)

		return c.Render(http.StatusOK, "", view.PageLogout())
	}
}
