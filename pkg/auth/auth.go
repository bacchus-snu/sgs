package auth

import (
	"context"
	"encoding/gob"
	"errors"
	"slices"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/spf13/viper"
	"golang.org/x/oauth2"
)

// We expect returned values to be stored in gorilla sessions, so they must be
// registered.
func init() {
	gob.Register(&User{})
	gob.Register(&oidcVerififer{})
}

type User struct {
	Username string   `json:"username"`
	Email    string   `json:"email"`
	Groups   []string `json:"groups"`
}

func (u *User) IsAdmin() bool {
	return slices.Contains(u.Groups, "bacchus")
}

type Config struct {
	Issuer       string   `mapstructure:"issuer"`
	ClientID     string   `mapstructure:"client_id"`
	ClientSecret string   `mapstructure:"client_secret"`
	Scopes       []string `mapstructure:"scopes"`
	RedirectURL  string   `mapstructure:"redirect_url"`
}

func (c *Config) Bind() {
	viper.BindEnv("auth.issuer", "SGS_AUTH_ISSUER")
	viper.BindEnv("auth.client_id", "SGS_AUTH_CLIENT_ID")
	viper.BindEnv("auth.client_secret", "SGS_AUTH_CLIENT_SECRET")
	viper.BindEnv("auth.scopes", "SGS_AUTH_SCOPES")
	viper.BindEnv("auth.redirect_url", "SGS_AUTH_REDIRECT_URL")

	viper.SetDefault("auth.scopes", []string{"openid", "profile", "email"})
}

func (c *Config) Validate() error {
	var err error

	if c.Issuer == "" {
		err = errors.Join(err, errors.New("issuer is required"))
	}
	if c.ClientID == "" {
		err = errors.Join(err, errors.New("client_id is required"))
	}
	if c.RedirectURL == "" {
		err = errors.Join(err, errors.New("redirect_url is required"))
	}

	return err
}

type Service interface {
	AuthURL() (url string, state any)
	Exchange(ctx context.Context, code, state string, verifier any) (*User, error)
}

type service struct {
	clientID     string
	clientSecret string
	scopes       []string

	config   *oauth2.Config
	provider *oidc.Provider
}

var _ Service = (*service)(nil)

func New(ctx context.Context, cfg Config) (*service, error) {
	provider, err := oidc.NewProvider(context.Background(), cfg.Issuer)
	if err != nil {
		return nil, err
	}

	oauthCfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint:     provider.Endpoint(),
		Scopes:       cfg.Scopes,
		RedirectURL:  cfg.RedirectURL,
	}

	return &service{
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		scopes:       cfg.Scopes,
		provider:     provider,
		config:       oauthCfg,
	}, nil
}

type oidcVerififer struct {
	// oidcVerififer must be serializable, thus we export all fields
	State    string
	Verifier string
}

func (svc *service) AuthURL() (string, any) {
	ver := oidcVerififer{
		State:    oauth2.GenerateVerifier(),
		Verifier: oauth2.GenerateVerifier(),
	}
	authURL := svc.config.
		AuthCodeURL(
			ver.State,
			oauth2.AccessTypeOffline,
			oauth2.S256ChallengeOption(ver.Verifier),
		)
	return authURL, &ver
}

func (svc *service) Exchange(ctx context.Context, code, state string, verifier any) (*User, error) {
	ver, ok := verifier.(*oidcVerififer)
	if !ok {
		return nil, errors.New("invalid verifier")
	}
	if state != ver.State {
		return nil, errors.New("state mismatch")
	}

	token, err := svc.config.
		Exchange(ctx, code, oauth2.VerifierOption(ver.Verifier))
	if err != nil {
		return nil, err
	}

	rawIDToken, _ := token.Extra("id_token").(string)
	idToken, err := svc.provider.
		VerifierContext(ctx, &oidc.Config{ClientID: svc.clientID}).
		Verify(ctx, rawIDToken)
	if err != nil {
		return nil, err
	}

	var user User
	if err := idToken.Claims(&user); err != nil {
		return nil, err
	}

	// Fetch additional claims from userinfo endpoint (email is not in ID token)
	userInfo, err := svc.provider.UserInfo(ctx, oauth2.StaticTokenSource(token))
	if err != nil {
		return nil, err
	}
	var extraClaims struct {
		Email string `json:"email"`
	}
	if err := userInfo.Claims(&extraClaims); err != nil {
		return nil, err
	}
	user.Email = extraClaims.Email

	// sanity check, ensure users are valid
	if user.Username == "" {
		return nil, errors.New("invalid user")
	}

	return &user, nil
}
