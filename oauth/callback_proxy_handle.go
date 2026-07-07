package oauth

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

func (p *OauthProvider) CallbackProxyHandle() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var flowId = r.Header.Get("x-flow-id")
		if flowId == "" {
			err = errors.New("Empty flowId")
			p.oauth.sendError(w, r, fmt.Errorf("empty flow id: %w", err), 0)
			return
		}
		var bodyBytes []byte
		if bodyBytes, err = io.ReadAll(r.Body); err != nil {
			p.oauth.sendError(w, r, fmt.Errorf("read body: %w", err), 0)
			return
		}
		defer r.Body.Close()
		var body RefreshTokenBody
		if err = json.Unmarshal(bodyBytes, &body); err != nil {
			p.oauth.sendError(w, r, fmt.Errorf("parse body: %w", err), 0)
			return
		}
		var oauthState oauthState
		if oauthState, err = p.oauth.getOauthState(r.Context(), flowId); err != nil {
			p.oauth.sendError(w, r, fmt.Errorf("get flow state: %w", err), 0)
			return
		}
		if !safeCompare(oauth2.S256ChallengeFromVerifier(body.CodeVerifier), oauthState.ClientCodeChallenge) {
			p.oauth.sendError(w, r, fmt.Errorf("the code verifier is bad"), 0)
			return
		}
		if !safeCompare(oauthState.State, body.State) {
			p.oauth.sendError(w, r, fmt.Errorf("the state is not the same"), 0)
			return
		}
		var tokenInfo TokenInfo
		if tokenInfo, err = p.exchangeToken(r.Context(), body.Code, oauthState.CodeVerifier, oauthState.CallbackUrl); err != nil {
			p.oauth.sendError(w, r, fmt.Errorf("get token: %w", err), 0)
			return
		}
		var verifiedIdToken *oidc.IDToken
		if verifiedIdToken, err = p.verifyIdToken(r.Context(), tokenInfo.IdToken); err != nil {
			p.oauth.sendError(w, r, fmt.Errorf("validate oidc: %w", err), 0)
			return
		}
		if verifiedIdToken != nil && !safeCompare(verifiedIdToken.Nonce, oauthState.Nonce) {
			p.oauth.sendError(w, r, fmt.Errorf("validate nonce, expected: %s, received: %s", oauthState.Nonce, verifiedIdToken.Nonce), 0)
			return
		}
		var user User
		if p.getUser != nil {
			if user, err = p.getUser(r.Context(), tokenInfo.AccessToken, p.oauth.apiClient); err != nil {
				p.oauth.sendError(w, r, fmt.Errorf("get user: %w", err), 0)
				return
			}
		}
		var sessionToken SessionToken
		if p.createSession != nil {
			if sessionToken, err = p.createSession(r.Context(), tokenInfo, user, verifiedIdToken); err != nil {
				p.oauth.sendError(w, r, fmt.Errorf("create session: %w", err), 0)
				return
			}
		} else if verifiedIdToken != nil {
			sessionToken = SessionToken{
				Token:   tokenInfo.IdToken,
				Expires: getIdTokenExpires(verifiedIdToken),
			}
		} else {
			sessionToken = SessionToken{
				Token:   tokenInfo.AccessToken,
				Expires: tokenInfo.ExpiresIn,
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sessionToken)

	}
}
