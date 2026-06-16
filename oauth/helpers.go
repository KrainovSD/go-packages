package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

func CreateSessionFromIdToken(tokenInfo TokenInfo, user User) (SessionToken, error) {
	return SessionToken{
		Token:   tokenInfo.IdToken,
		Expires: getIdTokenExpires(tokenInfo.IdToken, tokenInfo.ExpiresIn),
	}, nil
}
func CreateSessionFromAccessToken(tokenInfo TokenInfo, user User) (SessionToken, error) {
	return SessionToken{
		Token:   tokenInfo.AccessToken,
		Expires: tokenInfo.ExpiresIn,
	}, nil
}

func getIdTokenExpires(token string, expires int) int {
	var idExpires = expires
	var parts = strings.Split(token, ".")
	if len(parts) == 3 {
		var err error
		var payload []byte
		if payload, err = base64.RawURLEncoding.DecodeString(parts[1]); err == nil {
			var claims IdTokenClaim
			if err = json.Unmarshal(payload, &claims); err == nil {
				idExpires = claims.Exp - claims.Iat
			}
		}
	}
	return idExpires
}

type StateStore struct {
	store map[string]string
	mutex sync.RWMutex
}

var stateStore = StateStore{
	store: map[string]string{},
	mutex: sync.RWMutex{},
}

func (s *StateStore) Get(key string) string {
	s.mutex.Lock()
	result := s.store[key]
	delete(s.store, key)
	s.mutex.Unlock()

	return result
}
func (s *StateStore) Set(key string, value string) {
	s.mutex.Lock()
	s.store[key] = value
	s.mutex.Unlock()
}

func generateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))

	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func safeCompare(a string, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

type audienceList []string

func (a *audienceList) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*a = []string{single}
		return nil
	}
	var multi []string
	if err := json.Unmarshal(data, &multi); err != nil {
		return err
	}
	*a = multi
	return nil
}

type jwtPayload struct {
	Iss   string       `json:"iss"`
	Sub   string       `json:"sub"`
	Aud   audienceList `json:"aud"`
	Exp   int64        `json:"exp"`
	Iat   int64        `json:"iat"`
	Nonce string       `json:"nonce"`
	Jti   string       `json:"jti"`
}

func decodeJWT(token string) (jwtPayload, error) {
	var payload jwtPayload
	var parts = strings.Split(token, ".")
	if len(parts) != 3 {
		return payload, errors.New("bad jwt format")
	}
	var payloadBytes, err = base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return payload, fmt.Errorf("error decoding payload: %w", err)
	}
	if err = json.Unmarshal(payloadBytes, &payload); err != nil {
		return payload, fmt.Errorf("error parse payload: %w", err)
	}
	return payload, nil
}

type oauthState struct {
	State         string
	TimeKey       string
	Nonce         string
	CodeVerifier  string
	CodeChallenge string
}

func generateOauthServiceState() (oauthState, error) {
	var err error
	var servicesState oauthState

	state, err := randomHex(32)
	if err != nil {
		return servicesState, fmt.Errorf("generate state: %w", err)
	}
	timeKey, err := randomHex(32)
	if err != nil {
		return servicesState, fmt.Errorf("generate timeKey: %w", err)
	}
	nonce, err := randomBase64(32)
	if err != nil {
		return servicesState, fmt.Errorf("generate nonce: %w", err)
	}
	codeVerifier, err := randomHex(64)
	if err != nil {
		return servicesState, fmt.Errorf("generate codeVerifier: %w", err)
	}
	codeChallenge := generateCodeChallenge(codeVerifier)

	servicesState = oauthState{
		State:         state,
		TimeKey:       timeKey,
		Nonce:         nonce,
		CodeVerifier:  codeVerifier,
		CodeChallenge: codeChallenge,
	}

	return servicesState, nil
}

func setLogoutState(ctx context.Context, logoutState logoutState, key string, serviceDataExpires int, redisClient redis.UniversalClient) error {
	var logoutStateBytes []byte
	var err error
	if logoutStateBytes, err = json.Marshal(logoutState); err != nil {
		return fmt.Errorf("marshal logout state: %w", err)
	}
	if redisClient == nil {
		stateStore.Set(key, string(logoutStateBytes))
	} else {
		var cmd = redisClient.Set(ctx, key, string(logoutStateBytes), time.Duration(serviceDataExpires)*time.Second)
		if err = cmd.Err(); err != nil {
			return fmt.Errorf("set logout state in redis: %w", err)
		}
	}
	return nil
}

func getLogoutState(ctx context.Context, key string, redisClient redis.UniversalClient) (logoutState, error) {
	var state logoutState
	if key == "" {
		return state, fmt.Errorf("empty key")
	}
	var err error
	var stateStr string
	if redisClient == nil {
		stateStr = stateStore.Get(key)
	} else {
		var cmd = redisClient.Get(ctx, key)
		if stateStr, err = cmd.Result(); err != nil {
			return state, fmt.Errorf("get logout state from redis: %w", err)
		}
		redisClient.Del(ctx, key)
	}

	if err = json.Unmarshal([]byte(stateStr), &state); err != nil {
		return state, fmt.Errorf("parse logout state: %w", err)

	}

	return state, nil
}

func setFlowState(ctx context.Context, flowState flowState, key string, serviceDataExpires int, redisClient redis.UniversalClient) error {
	var flowStateBytes []byte
	var err error

	if flowStateBytes, err = json.Marshal(flowState); err != nil {
		return fmt.Errorf("parse flow state: %w", err)
	}

	if redisClient == nil {
		stateStore.Set(key, string(flowStateBytes))
	} else {
		var cmd = redisClient.Set(ctx, key, string(flowStateBytes), time.Duration(serviceDataExpires)*time.Second)
		if err = cmd.Err(); err != nil {
			return fmt.Errorf("set flow state in redis: %w", err)
		}
	}

	return nil
}

func getFlowState(ctx context.Context, key string, redisClient redis.UniversalClient) (flowState, error) {
	var fstate flowState
	if key == "" {
		return fstate, fmt.Errorf("empty key")
	}
	var err error
	var fstateStr string
	if redisClient == nil {
		fstateStr = stateStore.Get(key)
	} else {
		var cmd = redisClient.Get(ctx, key)
		if fstateStr, err = cmd.Result(); err != nil {
			return fstate, fmt.Errorf("get flow state from redis: %w", err)
		}
		redisClient.Del(ctx, key)
	}
	if err = json.Unmarshal([]byte(fstateStr), &fstate); err != nil {
		return fstate, fmt.Errorf("parse flow state: %w", err)

	}
	return fstate, nil
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

func generateLogUrl(options authUrlOptions) (string, error) {
	var logUrl *url.URL
	var err error

	if logUrl, err = url.Parse(options.Url); err != nil {
		return "", fmt.Errorf("parse login url: %w", err)
	}
	query := logUrl.Query()
	query.Add("nonce", options.Nonce)
	// with pkce
	if options.CodeChallenge != "" {
		query.Add("code_challenge", options.CodeChallenge)
		query.Add("code_challenge_method", "S256")
	}
	query.Add("state", options.State)
	query.Add("client_id", options.ClientId)
	query.Add("response_type", "code")
	query.Add("redirect_uri", options.CallbackUrl)
	if len(options.Scopes) > 0 {
		query.Add("scope", strings.Join(options.Scopes, " "))
	}
	logUrl.RawQuery = query.Encode()

	return logUrl.String(), nil
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

func randomHex(length int) (string, error) {
	bytes := make([]byte, (length+1)/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes), nil
}

func randomBase64(length int) (string, error) {
	bytes := make([]byte, (length+1)/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(bytes), nil
}
