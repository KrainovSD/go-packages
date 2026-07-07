package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/KrainovSD/go-packages/api"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/redis/go-redis/v9"
	"golang.org/x/oauth2"
)

type Oauth struct {
	redis                      redis.UniversalClient
	log                        *slog.Logger
	apiClient                  *api.Client
	cookieTimeKey              *Cookie
	cookieRefreshToken         *Cookie
	cookieSessionToken         *Cookie
	frontendErrorPath          string
	frontendClearPath          string
	frontendLogoutPath         string
	queryExpires               string
	stateLength                int
	serviceDataExpires         int
	defaultRefreshTokenExpires int
	frontendHost               string
	frontendProtocol           string
	updateToken                func(ctx context.Context, token string) (SessionToken, error)
	logout                     func(ctx context.Context, token string) error
}

type OauthOptions struct {
	Redis                      redis.UniversalClient
	ApiClient                  *api.Client
	Log                        *slog.Logger
	CookieTimeKey              *Cookie
	CookieRefreshToken         *Cookie
	CookieSessionToken         *Cookie
	FrontendClearPath          string
	FrontendErrorPath          string
	FrontendLogoutPath         string
	QueryExpires               string
	StateLength                int
	ServiceDataExpires         int
	DefaultRefreshTokenExpires int
	FrontendHost               string
	FrontendProtocol           string
	UpdateToken                func(ctx context.Context, token string) (SessionToken, error)
	Logout                     func(ctx context.Context, token string) error
}

func New(options *OauthOptions) (*Oauth, error) {
	var err error

	if err = options.validate(); err != nil {
		return nil, err
	}

	return &Oauth{
		redis:                      options.Redis,
		apiClient:                  options.ApiClient,
		log:                        options.Log,
		cookieTimeKey:              options.CookieTimeKey,
		cookieRefreshToken:         options.CookieRefreshToken,
		cookieSessionToken:         options.CookieSessionToken,
		frontendClearPath:          options.FrontendClearPath,
		frontendErrorPath:          options.FrontendErrorPath,
		frontendLogoutPath:         options.FrontendLogoutPath,
		queryExpires:               options.QueryExpires,
		stateLength:                options.StateLength,
		serviceDataExpires:         options.ServiceDataExpires,
		frontendHost:               options.FrontendHost,
		frontendProtocol:           options.FrontendProtocol,
		updateToken:                options.UpdateToken,
		logout:                     options.Logout,
		defaultRefreshTokenExpires: options.DefaultRefreshTokenExpires,
	}, nil
}

func (o *OauthOptions) validate() error {
	if o == nil {
		return errors.New("oauthOptions pointer is nil")
	}
	if o.Log == nil {
		return errors.New("log pointer is nil")
	}
	if o.CookieTimeKey == nil {
		o.CookieTimeKey = &Cookie{
			Prefix: "/",
			Name:   "session_time_key",
		}
	}
	if o.CookieTimeKey.Name == "" {
		o.CookieTimeKey.Name = "session_time_key"
	}
	if o.CookieTimeKey.Prefix == "" {
		o.CookieTimeKey.Prefix = "/"
	}
	if o.CookieRefreshToken != nil {
		if o.CookieRefreshToken.Name == "" {
			o.CookieRefreshToken.Name = "session_refresh_token"
		}
		if o.CookieRefreshToken.Prefix == "" {
			o.CookieRefreshToken.Prefix = "/"
		}
	}
	if o.CookieSessionToken != nil {
		if o.CookieSessionToken.Name == "" {
			o.CookieSessionToken.Name = "session_token"
		}
		if o.CookieSessionToken.Prefix == "" {
			o.CookieSessionToken.Prefix = "/"
		}
	}
	if o.QueryExpires == "" {
		o.QueryExpires = "session_token_expires"
	}
	if o.StateLength == 0 {
		o.StateLength = 16
	}
	if o.ServiceDataExpires == 0 {
		o.ServiceDataExpires = 5 * 60
	}
	if o.FrontendClearPath == "" {
		o.FrontendClearPath = "/clear"
	}
	if o.FrontendLogoutPath == "" {
		o.FrontendLogoutPath = "/logout"
	}
	if o.FrontendErrorPath == "" {
		o.FrontendErrorPath = "/error"
	}

	return nil
}

func (o *Oauth) extractToken(r *http.Request, cookieInfo *Cookie) (string, error) {
	var token string
	var err error
	token = r.Header.Get("Authorization")
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
		frontendHost = getHost(options.r, o.frontendHost)
	}
	if frontendProto == "" {
		frontendProto = getProto(options.r, o.frontendProtocol)
	}
	var comebackUrl = options.comebackUrl
	if comebackUrl == "" {
		if frontendProto != "" && frontendHost != "" {
			comebackUrl = frontendProto + "://" + frontendHost + o.frontendErrorPath
		} else {
			comebackUrl = o.frontendErrorPath
		}
	}
	http.Redirect(options.w, options.r, comebackUrl, http.StatusTemporaryRedirect)
}

func (o *Oauth) oauthProviderContext(ctx context.Context) context.Context {
	if o.apiClient != nil {
		return context.WithValue(ctx, oauth2.HTTPClient, o.apiClient.Client())
	}
	return ctx
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
		var cmd = o.redis.Set(ctx, key, string(oauthStateBytes), time.Duration(o.serviceDataExpires)*time.Second)
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

type OauthProvider struct {
	oauth           *Oauth
	oidcProvider    *oidc.Provider        // can be nil
	idTokenVerifier *oidc.IDTokenVerifier // can be nil
	config          *oauth2.Config
	loginPath       string
	tokenPath       string
	userPath        string
	logoutPath      string
	startAuthPath   string
	callbackPath    string
	clearPath       string
	provider        string
	parseUser       func(ctx context.Context, response []byte) (User, error)
	parseToken      func(ctx context.Context, response []byte) (TokenInfo, error)
	createSession   func(ctx context.Context, token TokenInfo, user User, verifiedIdToken *oidc.IDToken) (SessionToken, error)
}

type OauthProviderOptions struct {
	ClientId                   string
	ClientSecret               string
	Issuer                     string
	LoginPath                  string
	TokenPath                  string
	UserPath                   string
	LogoutPath                 string
	StartAuthPath              string
	CallbackPath               string
	ClearPath                  string
	Provider                   string
	ParseUser                  func(ctx context.Context, response []byte) (User, error)
	ParseToken                 func(ctx context.Context, response []byte) (TokenInfo, error)
	CreateSession              func(ctx context.Context, token TokenInfo, user User, verifiedIdToken *oidc.IDToken) (SessionToken, error)
	Scopes                     []string
	SkipClientIDCheck          bool
	SkipExpiryCheck            bool
	SkipIssuerCheck            bool
	InsecureSkipSignatureCheck bool
}

func (o *Oauth) NewProvider(options *OauthProviderOptions) (*OauthProvider, error) {
	var err error
	var provider *OauthProvider

	if o == nil {
		return provider, errors.New("oauth pointer is nil")
	}

	var oidcProvider *oidc.Provider
	var idTokenVerifier *oidc.IDTokenVerifier

	if options.Issuer != "" {
		var providerCtx = o.oauthProviderContext(context.Background())
		if oidcProvider, err = oidc.NewProvider(providerCtx, options.Issuer); err != nil {
			return provider, fmt.Errorf("oidc discovery: %w", err)
		}
		var claims oidcProviderClaims
		if err = oidcProvider.Claims(&claims); err == nil && claims.EndSessionEndpoint != "" && options.LogoutPath == "" {
			options.LogoutPath = claims.EndSessionEndpoint
		}
		var endpoint = oidcProvider.Endpoint()
		if options.LoginPath == "" {
			options.LoginPath = endpoint.AuthURL
		}
		if options.TokenPath == "" {
			options.TokenPath = endpoint.TokenURL
		}
		if options.UserPath == "" {
			options.UserPath = oidcProvider.UserInfoEndpoint()
		}

		idTokenVerifier = oidcProvider.Verifier(&oidc.Config{
			ClientID:                   options.ClientId,
			SkipClientIDCheck:          options.SkipClientIDCheck,
			SkipExpiryCheck:            options.SkipExpiryCheck,
			SkipIssuerCheck:            options.SkipIssuerCheck,
			InsecureSkipSignatureCheck: options.InsecureSkipSignatureCheck,
		})
	}

	if err = options.validate(); err != nil {
		return provider, fmt.Errorf("validate oauth provider: %w", err)
	}

	return &OauthProvider{
		oauth:           o,
		oidcProvider:    oidcProvider,
		idTokenVerifier: idTokenVerifier,
		config: &oauth2.Config{
			ClientID:     options.ClientId,
			ClientSecret: options.ClientSecret,
			Endpoint: oauth2.Endpoint{
				AuthURL:  options.LoginPath,
				TokenURL: options.TokenPath,
			},
			Scopes: options.Scopes,
		},
		provider:      options.Provider,
		loginPath:     options.LoginPath,
		tokenPath:     options.TokenPath,
		logoutPath:    options.LogoutPath,
		userPath:      options.UserPath,
		startAuthPath: options.StartAuthPath,
		callbackPath:  options.CallbackPath,
		clearPath:     options.ClearPath,
		parseUser:     options.ParseUser,
		parseToken:    options.ParseToken,
		createSession: options.CreateSession,
	}, nil
}

func (o *OauthProviderOptions) validate() error {
	if o == nil {
		return errors.New("oauthRegisterOptions pointer is nil")
	}
	if o.ClientId == "" {
		return errors.New("clientId is empty")
	}
	if o.ClientSecret == "" {
		return errors.New("clientSecret is empty")
	}
	if o.LoginPath == "" {
		return errors.New("loginPath is empty")
	}
	if o.TokenPath == "" {
		return errors.New("tokenPath is empty")
	}
	if o.UserPath == "" {
		return errors.New("userPath is empty")
	}
	if o.LogoutPath == "" {
		return errors.New("logoutPath is empty")
	}
	if o.CallbackPath == "" {
		return errors.New("callbackPath is empty")
	}
	if o.ClearPath == "" {
		return errors.New("clearPath is empty")
	}
	if o.StartAuthPath == "" {
		return errors.New("startAuthPath is empty")
	}
	if o.Provider == "" {
		return errors.New("provider is empty")
	}

	return nil
}

func (p *OauthProvider) exchangeTokenByRefresh(ctx context.Context, refreshToken string) (TokenInfo, error) {
	var err error
	var oauthToken *oauth2.Token
	if oauthToken, err = p.config.TokenSource(p.oauth.oauthProviderContext(ctx), &oauth2.Token{
		RefreshToken: refreshToken,
	}).Token(); err != nil {
		return TokenInfo{}, fmt.Errorf("request token: %w", err)
	}
	return p.newTokenInfo(oauthToken), nil
}

func (p *OauthProvider) exchangeToken(ctx context.Context, code string, codeVerifier string, callbackUrl string) (TokenInfo, error) {
	var err error
	var config = *p.config
	config.RedirectURL = callbackUrl
	var opts = []oauth2.AuthCodeOption{
		oauth2.VerifierOption(codeVerifier),
	}
	var oauthToken *oauth2.Token
	if oauthToken, err = config.Exchange(p.oauth.oauthProviderContext(ctx), code, opts...); err != nil {
		return TokenInfo{}, fmt.Errorf("request token: %w", err)
	}
	return p.newTokenInfo(oauthToken), nil
}

func (p *OauthProvider) newTokenInfo(oauthToken *oauth2.Token) TokenInfo {
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
		if p.oauth.defaultRefreshTokenExpires != 0 {
			token.RefreshTokenExpiresIn = p.oauth.defaultRefreshTokenExpires
		} else {
			token.RefreshTokenExpiresIn = token.ExpiresIn * 10
		}
	}
	return token
}

func (o *OauthProvider) getUser(ctx context.Context, token string, client *api.Client) (User, error) {
	var response api.Response
	var err error
	var user User

	if client == nil {
		return user, fmt.Errorf("client is nil")
	}

	if response, err = client.Send(api.Request{
		Url:         o.userPath,
		Method:      api.METHOD_GET,
		ContentType: api.CONTENT_TYPE_JSON,
		Headers:     map[string]string{"Authorization": "Bearer " + token},
	}); err != nil {

		return user, fmt.Errorf("request user: %w", err)
	}

	if user, err = o.parseUser(ctx, response.Data); err != nil {
		return user, fmt.Errorf("parse user: %w", err)
	}

	return user, nil
}

func (p *OauthProvider) verifyIdToken(ctx context.Context, rawIdToken string) (*oidc.IDToken, error) {
	if p.idTokenVerifier == nil || rawIdToken == "" {
		return nil, nil
	}
	var idToken *oidc.IDToken
	var err error
	if idToken, err = p.idTokenVerifier.Verify(ctx, rawIdToken); err != nil {
		return nil, fmt.Errorf("verify id token: %w", err)
	}
	return idToken, nil
}
