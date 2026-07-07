package oauth

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"net/url"

	"golang.org/x/oauth2"

	"github.com/KrainovSD/go-packages/helpers"
	"github.com/coreos/go-oidc/v3/oidc"
)

func CreateSessionFromIdToken(tokenInfo TokenInfo, user User, verifiedIdToken *oidc.IDToken) (SessionToken, error) {
	return SessionToken{
		Token:   tokenInfo.IdToken,
		Expires: getIdTokenExpires(verifiedIdToken),
	}, nil
}
func CreateSessionFromAccessToken(tokenInfo TokenInfo, user User) (SessionToken, error) {
	return SessionToken{
		Token:   tokenInfo.AccessToken,
		Expires: tokenInfo.ExpiresIn,
	}, nil
}

func getIdTokenExpires(token *oidc.IDToken) int {
	if token == nil {
		return 0
	}
	return int(token.Expiry.Sub(token.IssuedAt).Seconds())
}

func safeCompare(a string, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

type oauthStateOptions struct {
	CallbackUrl         string
	ComebackUrl         string
	ClientCodeChallenge string
}

func newOauthState(opts oauthStateOptions) (oauthState, string, error) {
	var state, err = helpers.RandomHex(32)
	if err != nil {
		return oauthState{}, "", fmt.Errorf("generate state: %w", err)
	}
	var timeKey string
	if timeKey, err = helpers.RandomHex(32); err != nil {
		return oauthState{}, timeKey, fmt.Errorf("generate timeKey: %w", err)
	}
	var nonce string
	if nonce, err = helpers.RandomBase64(32); err != nil {
		return oauthState{}, timeKey, fmt.Errorf("generate nonce: %w", err)
	}
	var codeVerifier = oauth2.GenerateVerifier()
	return oauthState{
		State:               state,
		Nonce:               nonce,
		CodeVerifier:        codeVerifier,
		ClientCodeChallenge: opts.ClientCodeChallenge,
		CallbackUrl:         opts.CallbackUrl,
		ComebackUrl:         opts.ComebackUrl,
	}, timeKey, nil
}

type authUrlOptions struct {
	Url           string
	Nonce         string
	CodeChallenge string
	State         string
	ClientId      string
	CallbackUrl   string
	Scopes        []string
}

func generateLogoutUrl(baseUrl string, comebackUrl string, tokenId string, clientId string) (string, error) {
	var logoutUrl *url.URL
	var err error

	if logoutUrl, err = url.Parse(baseUrl); err != nil {
		return "", fmt.Errorf("parse logout url: %w", err)
	}
	query := logoutUrl.Query()
	query.Add("id_token_hint", tokenId)
	query.Add("client_id", clientId)
	query.Add("post_logout_redirect_uri", comebackUrl)
	logoutUrl.RawQuery = query.Encode()

	return logoutUrl.String(), nil
}

func generateFallbackLogoutUrl(proto string, host string, startAuthPath string, frontendLogoutPath string) (string, error) {
	var fallbackLogoutUrl *url.URL
	var err error

	if fallbackLogoutUrl, err = url.Parse(proto + "://" + host + startAuthPath); err != nil {
		return "", fmt.Errorf("parse base url: %w", err)
	}
	query := fallbackLogoutUrl.Query()
	query.Add("frontend_protocol", proto)
	query.Add("frontend_host", host)
	query.Add("comeback_path", frontendLogoutPath)
	fallbackLogoutUrl.RawQuery = query.Encode()

	return fallbackLogoutUrl.String(), nil

}

func generateClearUrl(proto string, host string, clearPath string) (string, error) {
	var clearUrl *url.URL
	var err error
	if clearUrl, err = url.Parse(proto + "://" + host + clearPath); err != nil {
		return "", fmt.Errorf("parse base url: %w", err)
	}
	return clearUrl.String(), nil
}

func getProto(r *http.Request, custom string) string {
	var proto string
	var queryProtocol = r.URL.Query().Get("frontend_protocol")
	var proxyHeader = r.Header[http.CanonicalHeaderKey("x-forwarded-proto")]
	var scheme = r.URL.Scheme

	switch {
	case queryProtocol != "":
		proto = queryProtocol
	case custom != "":
		proto = custom
	case len(proxyHeader) > 0:
		proto = proxyHeader[0]
	case scheme != "":
		proto = scheme
	case r.TLS != nil:
		proto = "https"
	default:
		proto = "http"
	}

	return proto
}

func getHost(r *http.Request, custom string) string {
	var host string
	var queryHost = r.URL.Query().Get("frontend_host")

	switch {
	case queryHost != "":
		host = queryHost
	case custom != "":
		host = custom
	default:
		host = r.Host
	}

	return host
}
