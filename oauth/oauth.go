package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/KrainovSD/go-packages/api"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/redis/go-redis/v9"
	"golang.org/x/oauth2"
)

type OauthFrontend struct {
	ClearPath    string
	ErrorPath    string
	LogoutPath   string
	Host         string
	Protocol     string
	ExpiresQuery string
}

func (of *OauthFrontend) newDefault() *OauthFrontend {
	var o = *of
	if o.ClearPath == "" {
		o.ClearPath = "/clear"
	}
	if o.ErrorPath == "" {
		o.ErrorPath = "/error"
	}
	if o.LogoutPath == "" {
		o.LogoutPath = "/logout"
	}
	if o.ExpiresQuery == "" {
		o.ExpiresQuery = "session_token_expires"
	}
	return &o
}

type OauthSettings struct {
	StateLength                  int
	ServiceDataExpiresIn         int
	DefaultRefreshTokenExpiresIn int
}

func (os *OauthSettings) newDefault() *OauthSettings {
	var o = *os
	if o.StateLength == 0 {
		o.StateLength = 16
	}
	if o.ServiceDataExpiresIn == 0 {
		o.ServiceDataExpiresIn = 5 * 60
	}
	return &o
}

type OauthRouting struct {
	AuthPath     string
	CallbackPath string
	ClearPath    string
}

func (or *OauthRouting) validate() error {
	if or == nil {
		return errors.New("is nil")
	}
	if or.AuthPath == "" {
		return errors.New("auth path is empty")
	}
	if or.CallbackPath == "" {
		return errors.New("callback path is empty")
	}
	if or.ClearPath == "" {
		return errors.New("clear path is empty")
	}
	return nil
}

type OauthProvider struct {
	ClientID                   string
	ClientSecret               string
	Issuer                     string
	AuthURL                    string
	TokenURL                   string
	UserInfoURL                string
	EndSessionURL              string
	Scopes                     []string
	SkipClientIDCheck          bool
	SkipExpiryCheck            bool
	SkipIssuerCheck            bool
	InsecureSkipSignatureCheck bool
}

func (o *OauthProvider) validate() error {
	if o == nil {
		return errors.New("is nil")
	}
	if o.ClientID == "" {
		return errors.New("client id is empty")
	}
	if o.ClientSecret == "" {
		return errors.New("client secret is empty")
	}
	if o.Issuer == "" && (o.AuthURL == "" || o.TokenURL == "") {
		return errors.New("issuer and auth url or token url is empty")
	}
	return nil
}

type OauthHandlers struct {
	UpdateToken   func(ctx context.Context, token string) (SessionToken, error)
	EndSession    func(ctx context.Context, token string, URL string) error
	ParseUser     func(ctx context.Context, response []byte) (User, error)
	CreateSession func(ctx context.Context, token TokenInfo, user User, verifiedIdToken *oidc.IDToken) (SessionToken, error)
}

type OauthCookie struct {
	Prefix string
	Name   string
}

type OauthOptions struct {
	Redis              redis.UniversalClient
	Http               *http.Client
	Log                *slog.Logger
	Frontend           *OauthFrontend
	Settings           *OauthSettings
	Routing            *OauthRouting
	Provider           *OauthProvider
	Handlers           *OauthHandlers
	CookieTimeKey      *OauthCookie
	CookieSessionToken *OauthCookie
	CookieRefreshToken *OauthCookie
}

func (oo *OauthOptions) newDefault() *OauthOptions {
	var o = *oo
	if o.Frontend == nil {
		o.Frontend = &OauthFrontend{}
	}
	o.Frontend = o.Frontend.newDefault()
	if o.Settings == nil {
		o.Settings = &OauthSettings{}
	}
	o.Settings = o.Settings.newDefault()
	if o.Handlers == nil {
		o.Handlers = &OauthHandlers{}
	}
	if o.CookieTimeKey == nil {
		o.CookieTimeKey = &OauthCookie{}
	}
	if o.CookieTimeKey.Name == "" {
		o.CookieTimeKey.Name = "session_time_key"
	}
	if o.CookieTimeKey.Prefix == "" {
		o.CookieTimeKey.Prefix = "/"
	}
	if o.CookieSessionToken != nil {
		if o.CookieSessionToken.Name == "" {
			o.CookieSessionToken.Name = "session_token"
		}
		if o.CookieSessionToken.Prefix == "" {
			o.CookieSessionToken.Prefix = "/"
		}
	}
	if o.CookieRefreshToken != nil {
		if o.CookieRefreshToken.Name == "" {
			o.CookieRefreshToken.Name = "session_refresh_token"
		}
		if o.CookieRefreshToken.Prefix == "" {
			o.CookieRefreshToken.Prefix = "/"
		}
	}
	return &o
}

func (oo *OauthOptions) validate() error {
	if oo.Http == nil {
		return errors.New("http client is nil")
	}
	if oo.Log == nil {
		return errors.New("logger is nil")
	}
	var err error
	if err = oo.Routing.validate(); err != nil {
		return fmt.Errorf("routing: %w", err)
	}
	if err = oo.Provider.validate(); err != nil {
		return fmt.Errorf("provider: %w", err)
	}
	return nil
}

type Oauth struct {
	redis               redis.UniversalClient
	http                *http.Client
	log                 *slog.Logger
	frontend            *OauthFrontend
	settings            *OauthSettings
	routing             *OauthRouting
	provider            *oauth2.Config
	handlers            *OauthHandlers
	userInfoURL         string
	endSessionURL       string
	cookieTimeKey       *OauthCookie
	cookieSessionToken  *OauthCookie
	cookieRefreshToken  *OauthCookie
	oidcProvider        *oidc.Provider        // can be nil
	oidcIdTokenVerifier *oidc.IDTokenVerifier // can be nil
}

func New(o *OauthOptions) (*Oauth, error) {
	var err error
	if err = o.validate(); err != nil {
		return nil, fmt.Errorf("validate options: %w", err)
	}
	var opts = o.newDefault()
	var userInfoURL = opts.Provider.UserInfoURL
	var endSessionURL = opts.Provider.EndSessionURL
	var oauthProvider *oauth2.Config
	var oidcProvider *oidc.Provider
	var oidcIdTokenVerifier *oidc.IDTokenVerifier
	if opts.Provider.Issuer != "" {
		if oidcProvider, err = oidc.NewProvider(context.WithValue(context.Background(), oauth2.HTTPClient, opts.Http), opts.Provider.Issuer); err != nil {
			return nil, fmt.Errorf("new oidc provider: %w", err)
		}
		oidcIdTokenVerifier = oidcProvider.Verifier(&oidc.Config{
			ClientID:                   opts.Provider.ClientID,
			SkipClientIDCheck:          opts.Provider.SkipClientIDCheck,
			SkipExpiryCheck:            opts.Provider.SkipExpiryCheck,
			SkipIssuerCheck:            opts.Provider.SkipIssuerCheck,
			InsecureSkipSignatureCheck: opts.Provider.InsecureSkipSignatureCheck,
		})
		var endpoint = oidcProvider.Endpoint()
		oauthProvider = &oauth2.Config{
			ClientID:     opts.Provider.ClientID,
			ClientSecret: opts.Provider.ClientSecret,
			Endpoint:     endpoint,
			Scopes:       opts.Provider.Scopes,
		}
		if userInfoURL == "" {
			userInfoURL = oidcProvider.UserInfoEndpoint()
		}
		if endSessionURL == "" {
			var claims struct {
				EndSessionEndpoint string `json:"end_session_endpoint"`
			}
			if err := oidcProvider.Claims(&claims); err == nil && claims.EndSessionEndpoint != "" {
				endSessionURL = claims.EndSessionEndpoint
			}
		}
	} else {
		oauthProvider = &oauth2.Config{
			ClientID:     opts.Provider.ClientID,
			ClientSecret: opts.Provider.ClientSecret,
			Endpoint: oauth2.Endpoint{
				AuthURL:  opts.Provider.AuthURL,
				TokenURL: opts.Provider.TokenURL,
			},
			Scopes: opts.Provider.Scopes,
		}
	}
	return &Oauth{
		redis:               opts.Redis,
		http:                opts.Http,
		log:                 opts.Log,
		frontend:            opts.Frontend,
		settings:            opts.Settings,
		routing:             opts.Routing,
		provider:            oauthProvider,
		handlers:            opts.Handlers,
		userInfoURL:         userInfoURL,
		endSessionURL:       endSessionURL,
		cookieTimeKey:       opts.CookieTimeKey,
		cookieSessionToken:  opts.CookieSessionToken,
		cookieRefreshToken:  opts.CookieRefreshToken,
		oidcProvider:        oidcProvider,
		oidcIdTokenVerifier: oidcIdTokenVerifier,
	}, nil
}

func (o *Oauth) newOauthHttpContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, oauth2.HTTPClient, o.http)
}

func (o *Oauth) extractToken(r *http.Request, cookieInfo *OauthCookie) (string, error) {
	var err error
	var token = r.Header.Get("Authorization")
	if token != "" {
		token = strings.Replace(token, "Bearer ", "", 1)
	} else if cookieInfo != nil {
		var cookie *http.Cookie
		if cookie, err = r.Cookie(cookieInfo.Name); err != nil {
			return token, fmt.Errorf("get from cookie: %w", err)

		}
		token = cookie.Value
	}
	if token == "" {
		return token, fmt.Errorf("empty token")
	}

	return token, nil
}

func (o *Oauth) sendError(w http.ResponseWriter, r *http.Request, err error, status int) {
	if status == 0 {
		status = 409
	}
	o.log.LogAttrs(context.Background(), slog.LevelWarn, "oauth", slog.String("error", err.Error()))
	w.WriteHeader(status)
}

type redirectErrorOptions struct {
	w             http.ResponseWriter
	r             *http.Request
	err           error
	frontendHost  string
	frontendProto string
	comebackUrl   string
}

func (o *Oauth) redirectError(options redirectErrorOptions) {
	o.log.LogAttrs(context.Background(), slog.LevelWarn, "oauth", slog.String("error", options.err.Error()))
	var frontendHost = options.frontendHost
	var frontendProto = options.frontendProto
	if frontendHost == "" {
		frontendHost = getHost(options.r, o.frontend.Host)
	}
	if frontendProto == "" {
		frontendProto = getProto(options.r, o.frontend.Protocol)
	}
	var comebackUrl = options.comebackUrl
	if comebackUrl == "" {
		if frontendProto != "" && frontendHost != "" {
			comebackUrl = frontendProto + "://" + frontendHost + o.frontend.ErrorPath
		} else {
			comebackUrl = o.frontend.ErrorPath
		}
	}
	http.Redirect(options.w, options.r, comebackUrl, http.StatusTemporaryRedirect)
}

func (o *Oauth) setOauthState(ctx context.Context, oauthState oauthState, key string) error {
	var oauthStateBytes []byte
	var err error
	if oauthStateBytes, err = json.Marshal(oauthState); err != nil {
		return fmt.Errorf("parse flow state: %w", err)
	}
	if o.redis == nil {
		stateStore.Set(key, string(oauthStateBytes))
	} else {
		var cmd = o.redis.Set(ctx, key, string(oauthStateBytes), time.Duration(o.settings.ServiceDataExpiresIn)*time.Second)
		if err = cmd.Err(); err != nil {
			return fmt.Errorf("set flow state in redis: %w", err)
		}
	}
	return nil
}

func (o *Oauth) getOauthState(ctx context.Context, key string) (oauthState, error) {
	var oauthState oauthState
	if key == "" {
		return oauthState, fmt.Errorf("empty key")
	}
	var err error
	var oauthStateStr string
	if o.redis == nil {
		oauthStateStr = stateStore.Get(key)
	} else {
		var cmd = o.redis.Get(ctx, key)
		if oauthStateStr, err = cmd.Result(); err != nil {
			return oauthState, fmt.Errorf("get flow state from redis: %w", err)
		}
		o.redis.Del(ctx, key)
	}
	if err = json.Unmarshal([]byte(oauthStateStr), &oauthState); err != nil {
		return oauthState, fmt.Errorf("parse flow state: %w", err)

	}
	return oauthState, nil
}

func (o *Oauth) setLogoutState(ctx context.Context, logoutState logoutState, key string) error {
	var logoutStateBytes []byte
	var err error
	if logoutStateBytes, err = json.Marshal(logoutState); err != nil {
		return fmt.Errorf("marshal logout state: %w", err)
	}
	if o.redis == nil {
		stateStore.Set(key, string(logoutStateBytes))
	} else {
		var cmd = o.redis.Set(ctx, key, string(logoutStateBytes), time.Duration(o.settings.ServiceDataExpiresIn)*time.Second)
		if err = cmd.Err(); err != nil {
			return fmt.Errorf("set logout state in redis: %w", err)
		}
	}
	return nil
}

func (o *Oauth) getLogoutState(ctx context.Context, key string) (logoutState, error) {
	var state logoutState
	if key == "" {
		return state, fmt.Errorf("empty key")
	}
	var err error
	var stateStr string
	if o.redis == nil {
		stateStr = stateStore.Get(key)
	} else {
		var cmd = o.redis.Get(ctx, key)
		if stateStr, err = cmd.Result(); err != nil {
			return state, fmt.Errorf("get logout state from redis: %w", err)
		}
		o.redis.Del(ctx, key)
	}
	if err = json.Unmarshal([]byte(stateStr), &state); err != nil {
		return state, fmt.Errorf("parse logout state: %w", err)

	}
	return state, nil
}

func (o *Oauth) exchangeTokenByRefresh(ctx context.Context, refreshToken string) (TokenInfo, error) {
	var err error
	var oauthToken *oauth2.Token
	if oauthToken, err = o.provider.TokenSource(o.newOauthHttpContext(ctx), &oauth2.Token{
		RefreshToken: refreshToken,
	}).Token(); err != nil {
		return TokenInfo{}, fmt.Errorf("request token: %w", err)
	}
	return o.newTokenInfo(oauthToken), nil
}

func (o *Oauth) exchangeToken(ctx context.Context, code string, codeVerifier string, callbackUrl string) (TokenInfo, error) {
	var err error
	var provider = *o.provider
	provider.RedirectURL = callbackUrl
	var opts = []oauth2.AuthCodeOption{
		oauth2.VerifierOption(codeVerifier),
	}
	var oauthToken *oauth2.Token
	if oauthToken, err = provider.Exchange(o.newOauthHttpContext(ctx), code, opts...); err != nil {
		return TokenInfo{}, fmt.Errorf("request token: %w", err)
	}
	return o.newTokenInfo(oauthToken), nil
}

func (o *Oauth) newTokenInfo(oauthToken *oauth2.Token) TokenInfo {
	var token = TokenInfo{
		AccessToken:  oauthToken.AccessToken,
		RefreshToken: oauthToken.RefreshToken,
		ExpiresIn:    int(oauthToken.ExpiresIn),
	}
	var idTokenRaw = oauthToken.Extra("id_token")
	if idTokenRaw != nil {
		if idToken, ok := idTokenRaw.(string); ok {
			token.IdToken = idToken
		}
	}
	var refreshExpiresRaw = oauthToken.Extra("refresh_expires_in")
	if refreshExpiresRaw != nil {
		switch refreshExpiresIn := refreshExpiresRaw.(type) {
		case float64:
			token.RefreshTokenExpiresIn = int(refreshExpiresIn)
		case int:
			token.RefreshTokenExpiresIn = refreshExpiresIn
		}

	}
	if token.RefreshToken != "" && token.RefreshTokenExpiresIn == 0 {
		if o.settings.DefaultRefreshTokenExpiresIn != 0 {
			token.RefreshTokenExpiresIn = o.settings.DefaultRefreshTokenExpiresIn
		} else {
			token.RefreshTokenExpiresIn = token.ExpiresIn * 10
		}
	}
	return token
}

func (o *Oauth) getUser(ctx context.Context, token string) (User, error) {
	var err error
	var user User
	var req *http.Request
	if req, err = http.NewRequestWithContext(ctx, string(api.MethodPost), o.userInfoURL, nil); err != nil {
		return User{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", string(api.ContentTypeJSON))
	var res *http.Response
	if res, err = o.http.Do(req); err != nil {
		return User{}, fmt.Errorf("do request: %w", err)
	}
	defer res.Body.Close()
	var content []byte
	if content, err = io.ReadAll(io.LimitReader(res.Body, 10<<20)); err != nil {
		return User{}, fmt.Errorf("read request: %w", err)
	}
	if user, err = o.handlers.ParseUser(ctx, content); err != nil {
		return user, fmt.Errorf("parse user: %w", err)
	}
	return user, nil
}

func (o *Oauth) verifyIdToken(ctx context.Context, rawIdToken string) (*oidc.IDToken, error) {
	if o.oidcIdTokenVerifier == nil || rawIdToken == "" {
		return nil, nil
	}
	var idToken *oidc.IDToken
	var err error
	if idToken, err = o.oidcIdTokenVerifier.Verify(ctx, rawIdToken); err != nil {
		return nil, fmt.Errorf("verify id token: %w", err)
	}
	return idToken, nil
}

func (o *Oauth) clearCookies(w http.ResponseWriter, protocol string) {
	if o.cookieTimeKey != nil {
		http.SetCookie(w, &http.Cookie{
			Name:     o.cookieTimeKey.Name,
			Value:    "",
			Path:     o.cookieTimeKey.Prefix,
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   protocol == "https",
		})
	}
	if o.cookieRefreshToken != nil {
		http.SetCookie(w, &http.Cookie{
			Name:     o.cookieRefreshToken.Name,
			Value:    "",
			Path:     o.cookieRefreshToken.Prefix,
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   protocol == "https",
		})
	}
	if o.cookieSessionToken != nil {
		http.SetCookie(w, &http.Cookie{
			Name:     o.cookieSessionToken.Name,
			Value:    "",
			Path:     o.cookieSessionToken.Prefix,
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   protocol == "https",
		})
	}
}
