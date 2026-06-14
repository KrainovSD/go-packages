package oauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/KrainovSD/go-packages/api"
	"github.com/redis/go-redis/v9"
)

type Oauth struct {
	m                  *http.ServeMux
	redis              *redis.Client
	log                *slog.Logger
	apiClient          *api.Client
	cookieTimeKey      *Cookie
	cookieRefreshToken *Cookie
	cookieSessionToken *Cookie
	frontendErrorPath  string
	frontendClearPath  string
	frontendLogoutPath string
	queryExpires       string
	stateLength        int
	serviceDataExpires int
	frontendHost       string
	frontendProtocol   string
	updateToken        func(ctx context.Context, token string) (SessionToken, error)
	logout             func(ctx context.Context, token string) error
}

type OauthOptions struct {
	Redis              *redis.Client
	ApiClient          *api.Client
	Log                *slog.Logger
	CookieTimeKey      *Cookie
	CookieRefreshToken *Cookie
	CookieSessionToken *Cookie
	FrontendClearPath  string
	FrontendErrorPath  string
	FrontendLogoutPath string
	QueryExpires       string
	StateLength        int
	ServiceDataExpires int
	FrontendHost       string
	FrontendProtocol   string
	UpdateToken        func(ctx context.Context, token string) (SessionToken, error)
	Logout             func(ctx context.Context, token string) error
}

func Create(options *OauthOptions) (*Oauth, error) {
	var err error

	if err = options.validate(); err != nil {
		return nil, err
	}

	return &Oauth{
		redis:              options.Redis,
		apiClient:          options.ApiClient,
		log:                options.Log,
		cookieTimeKey:      options.CookieTimeKey,
		cookieRefreshToken: options.CookieRefreshToken,
		cookieSessionToken: options.CookieSessionToken,
		frontendClearPath:  options.FrontendClearPath,
		frontendErrorPath:  options.FrontendErrorPath,
		frontendLogoutPath: options.FrontendLogoutPath,
		queryExpires:       options.QueryExpires,
		stateLength:        options.StateLength,
		serviceDataExpires: options.ServiceDataExpires,
		frontendHost:       options.FrontendHost,
		frontendProtocol:   options.FrontendProtocol,
		updateToken:        options.UpdateToken,
		logout:             options.Logout,
	}, nil
}

func (o *OauthOptions) validate() error {
	if o == nil {
		return errors.New("oauthOptions pointer is nil")
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

type OauthProvider struct {
	oauth        *Oauth
	clientId     string
	clientSecret string
	issuer       string
	// oauth url
	loginPath string
	// oauth url
	tokenPath string
	// oauth url
	userPath string
	// oauth url
	logoutPath       string
	startAuthPath    string
	callbackPath     string
	clearPath        string
	provider         string
	parseUser        func(ctx context.Context, response []byte) (User, error)
	parseToken       func(ctx context.Context, response []byte) (TokenInfo, error)
	createSession    func(ctx context.Context, token TokenInfo, user User) (SessionToken, error)
	scopes           []string
	iatLeewaySeconds int
}

type OauthProviderOptions struct {
	ClientId     string
	ClientSecret string
	OidcPath     string
	// oauth url
	LoginPath string
	// oauth url
	TokenPath string
	// oauth url
	UserPath string
	// oauth url
	LogoutPath       string
	StartAuthPath    string
	CallbackPath     string
	ClearPath        string
	Provider         string
	ParseUser        func(ctx context.Context, response []byte) (User, error)
	ParseToken       func(ctx context.Context, response []byte) (TokenInfo, error)
	CreateSession    func(ctx context.Context, token TokenInfo, user User) (SessionToken, error)
	Scopes           []string
	IatLeewaySeconds int
}

func (o *Oauth) CreateOauthProvider(options *OauthProviderOptions) (*OauthProvider, error) {
	var err error
	var provider *OauthProvider

	if o == nil {
		return provider, errors.New("oauth pointer is nil")
	}

	var issuer string
	if options.OidcPath != "" {
		var err error
		var oidcConfig OidcConfig
		var oidcConfigBytes api.Response

		if oidcConfigBytes, err = o.apiClient.Send(api.Request{
			Url:    options.OidcPath,
			Method: api.METHOD_GET,
		}); err != nil {
			return provider, fmt.Errorf("get oauth config: %w", err)
		}

		if err = json.Unmarshal(oidcConfigBytes.Data, &oidcConfig); err != nil {
			return provider, fmt.Errorf("unmarshal oauth config: %w", err)
		}

		issuer = oidcConfig.Issuer
		if oidcConfig.LogoutPath != "" && options.LogoutPath == "" {
			options.LogoutPath = oidcConfig.LogoutPath
		}
		if oidcConfig.LoginPath != "" && options.LoginPath == "" {
			options.LoginPath = oidcConfig.LoginPath
		}
		if oidcConfig.UserPath != "" && options.UserPath == "" {
			options.UserPath = oidcConfig.UserPath
		}
		if oidcConfig.TokenPath != "" && options.TokenPath == "" {
			options.TokenPath = oidcConfig.TokenPath
		}
	}
	if err = options.validate(); err != nil {
		return provider, fmt.Errorf("validate oauth provider: %w", err)
	}

	return &OauthProvider{
		oauth:            o,
		clientId:         options.ClientId,
		clientSecret:     options.ClientSecret,
		issuer:           issuer,
		provider:         options.Provider,
		loginPath:        options.LoginPath,
		tokenPath:        options.TokenPath,
		userPath:         options.UserPath,
		logoutPath:       options.LogoutPath,
		startAuthPath:    options.StartAuthPath,
		callbackPath:     options.CallbackPath,
		clearPath:        options.ClearPath,
		parseUser:        options.ParseUser,
		parseToken:       options.ParseToken,
		createSession:    options.CreateSession,
		scopes:           options.Scopes,
		iatLeewaySeconds: options.IatLeewaySeconds,
	}, nil
}

func (o *OauthProviderOptions) validate() error {

	if o == nil {
		return errors.New("oauthRegisterOptions pointer is nil")
	}
	if o.IatLeewaySeconds == 0 {
		o.IatLeewaySeconds = 60
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

func (o *OauthProvider) getTokenByRefresh(ctx context.Context, apiClient *api.Client, refreshToken string) (TokenInfo, error) {
	var err error
	var tokenInfo TokenInfo

	formData := url.Values{}
	formData.Set("grant_type", "refresh_token")
	formData.Set("client_id", o.clientId)
	formData.Set("client_secret", o.clientSecret)
	formData.Set("refresh_token", refreshToken)

	var response api.Response
	if response, err = apiClient.Send(api.Request{
		Url:         o.tokenPath,
		Method:      api.METHOD_POST,
		ContentType: api.CONTENT_TYPE_FORM,
		Body:        bytes.NewBufferString(formData.Encode()),
	}); err != nil {
		return tokenInfo, fmt.Errorf("request token: %w", err)
	}

	if o.parseToken != nil {
		if tokenInfo, err = o.parseToken(ctx, response.Data); err != nil {
			return tokenInfo, fmt.Errorf("parse token: %w", err)
		}
	} else {
		var oidcToken OidcToken
		if err = json.Unmarshal(response.Data, &oidcToken); err != nil {
			return tokenInfo, fmt.Errorf("unmarshal oidc token: %w", err)
		}
		tokenInfo = TokenInfo{AccessToken: oidcToken.AccessToken, IdToken: oidcToken.IdToken, RefreshToken: oidcToken.RefreshToken, ExpiresIn: oidcToken.ExpiresIn, RefreshTokenExpiresIn: oidcToken.RefreshTokenExpiresIn}
	}

	return tokenInfo, nil
}

type getTokenOptions struct {
	CodeVerifier string
	Code         string
	CallbackUrl  string
	ApiClient    *api.Client
}

func (o *OauthProvider) getToken(ctx context.Context, options getTokenOptions) (TokenInfo, error) {
	var err error
	var tokenInfo TokenInfo

	formData := url.Values{}
	formData.Set("grant_type", "authorization_code")
	formData.Set("client_id", o.clientId)
	formData.Set("client_secret", o.clientSecret)
	if options.CodeVerifier != "" {
		formData.Set("code_verifier", options.CodeVerifier)
	}
	formData.Set("code", options.Code)
	formData.Set("redirect_uri", options.CallbackUrl)

	var response api.Response
	if response, err = options.ApiClient.Send(api.Request{
		Url:         o.tokenPath,
		Method:      api.METHOD_POST,
		ContentType: api.CONTENT_TYPE_FORM,
		Body:        bytes.NewBufferString(formData.Encode()),
	}); err != nil {
		return tokenInfo, fmt.Errorf("request token: %w", err)
	}

	if o.parseToken != nil {
		if tokenInfo, err = o.parseToken(ctx, response.Data); err != nil {
			return tokenInfo, fmt.Errorf("parse token: %w", err)
		}
	} else {
		var oidcToken OidcToken
		if err = json.Unmarshal(response.Data, &oidcToken); err != nil {
			return tokenInfo, fmt.Errorf("unmarshal oidc token: %w", err)
		}
		tokenInfo = TokenInfo{AccessToken: oidcToken.AccessToken, IdToken: oidcToken.IdToken, RefreshToken: oidcToken.RefreshToken, ExpiresIn: oidcToken.ExpiresIn, RefreshTokenExpiresIn: oidcToken.RefreshTokenExpiresIn}
	}

	return tokenInfo, nil
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

func (o *OauthProvider) oidcValidate(idToken string) error {
	if idToken == "" {
		return nil
	}
	var payload jwtPayload
	var err error
	if payload, err = decodeJWT(idToken); err != nil {
		return fmt.Errorf("decode jwt: %w", err)
	}
	var now = time.Now().Unix()
	if o.issuer != "" && payload.Iss != o.issuer {
		return fmt.Errorf("invalid issuer: got %q, want %q", payload.Iss, o.issuer)
	}
	if payload.Exp < now {
		return fmt.Errorf("token is expired")
	}
	if payload.Iat > 0 && payload.Iat > now+int64(o.iatLeewaySeconds) {
		return fmt.Errorf("token issued in the future: iat=%d, now=%d", payload.Iat, now)
	}
	var audMatch bool
	for _, aud := range payload.Aud {
		if safeCompare(aud, o.clientId) {
			audMatch = true
			break
		}
	}
	if !audMatch {
		return fmt.Errorf("bad client id")
	}
	return nil
}

func (o *OauthProvider) oidcFlowValidate(idToken string, nonce string) error {
	var err error
	if err = o.oidcValidate(idToken); err != nil {
		return err
	}
	if idToken == "" {
		return nil
	}
	var payload jwtPayload
	if payload, err = decodeJWT(idToken); err != nil {
		return fmt.Errorf("decode jwt: %w", err)
	}
	if !safeCompare(payload.Nonce, nonce) {
		return fmt.Errorf("bad nonce")
	}
	return nil
}
