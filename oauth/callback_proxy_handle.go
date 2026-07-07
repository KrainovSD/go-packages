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

func (o *Oauth) CallbackProxyHandle() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var flowId = r.Header.Get("x-flow-id")
		if flowId == "" {
			err = errors.New("Empty flowId")
			o.sendError(w, r, fmt.Errorf("empty flow id: %w", err), 0)
			return
		}
		var bodyBytes []byte
		if bodyBytes, err = io.ReadAll(r.Body); err != nil {
			o.sendError(w, r, fmt.Errorf("read body: %w", err), 0)
			return
		}
		defer r.Body.Close()
		var body RefreshTokenBody
		if err = json.Unmarshal(bodyBytes, &body); err != nil {
			o.sendError(w, r, fmt.Errorf("parse body: %w", err), 0)
			return
		}
		var oauthState oauthState
		if oauthState, err = o.getOauthState(r.Context(), flowId); err != nil {
			o.sendError(w, r, fmt.Errorf("get flow state: %w", err), 0)
			return
		}
		if !safeCompare(oauth2.S256ChallengeFromVerifier(body.CodeVerifier), oauthState.ClientCodeChallenge) {
			o.sendError(w, r, fmt.Errorf("the code verifier is bad"), 0)
			return
		}
		if !safeCompare(oauthState.State, body.State) {
			o.sendError(w, r, fmt.Errorf("the state is not the same"), 0)
			return
		}
		var tokenInfo TokenInfo
		if tokenInfo, err = o.exchangeToken(r.Context(), body.Code, oauthState.CodeVerifier, oauthState.CallbackUrl); err != nil {
			o.sendError(w, r, fmt.Errorf("get token: %w", err), 0)
			return
		}
		var verifiedIdToken *oidc.IDToken
		if verifiedIdToken, err = o.verifyIdToken(r.Context(), tokenInfo.IdToken); err != nil {
			o.sendError(w, r, fmt.Errorf("validate oidc: %w", err), 0)
			return
		}
		if verifiedIdToken != nil && !safeCompare(verifiedIdToken.Nonce, oauthState.Nonce) {
			o.sendError(w, r, fmt.Errorf("validate nonce, expected: %s, received: %s", oauthState.Nonce, verifiedIdToken.Nonce), 0)
			return
		}
		var user User
		if o.handlers.ParseUser != nil {
			if user, err = o.getUser(r.Context(), tokenInfo.AccessToken); err != nil {
				o.sendError(w, r, fmt.Errorf("get user: %w", err), 0)
				return
			}
		}
		var sessionToken SessionToken
		if o.handlers.CreateSession != nil {
			if sessionToken, err = o.handlers.CreateSession(r.Context(), tokenInfo, user, verifiedIdToken); err != nil {
				o.sendError(w, r, fmt.Errorf("create session: %w", err), 0)
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
