package oauth

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

func (p *OauthProvider) TokenProxyHandle() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		/** check flow id */
		var flowId = r.Header.Get("x-flow-id")
		if flowId == "" {
			err = errors.New("Empty flowId")
			p.oauth.sendError(w, r, fmt.Errorf("empty flow id: %w", err), 0)
			return
		}

		/** extract body */
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			p.oauth.sendError(w, r, fmt.Errorf("read body: %w", err), 0)
			return
		}
		defer r.Body.Close()
		var body RefreshTokenBody
		err = json.Unmarshal(bodyBytes, &body)
		if err != nil {
			p.oauth.sendError(w, r, fmt.Errorf("parse body: %w", err), 0)
			return
		}

		/** extract flow state */
		var fstate flowState
		if fstate, err = getFlowState(r.Context(), flowId, p.oauth.redis); err != nil {
			p.oauth.sendError(w, r, fmt.Errorf("get flow state: %w", err), 0)
			return
		}
		/** check PKCE code verifier */
		if !safeCompare(generateCodeChallenge(body.CodeVerifier), fstate.ClientCodeChallenge) {
			p.oauth.sendError(w, r, fmt.Errorf("the code verifier is bad"), 0)
			return
		}
		/** check state */
		if !safeCompare(fstate.State, body.State) {
			p.oauth.sendError(w, r, fmt.Errorf("the state is not the same"), 0)
			return
		}

		/** get access token */
		var tokenInfo TokenInfo
		if tokenInfo, err = p.getToken(r.Context(), getTokenOptions{
			CodeVerifier: fstate.CodeVerifier,
			Code:         body.Code,
			CallbackUrl:  fstate.CallbackUrl,
			ApiClient:    p.oauth.apiClient,
		}); err != nil {
			p.oauth.sendError(w, r, fmt.Errorf("get token: %w", err), 0)
			return
		}

		/** validate if OIDC protocol supported  */
		if err = p.oidcValidate(tokenInfo.IdToken); err != nil {
			p.oauth.sendError(w, r, fmt.Errorf("validate oidc: %w", err), 0)
			return
		}

		/** get user */
		var user User
		if user, err = p.getUser(r.Context(), tokenInfo.AccessToken, p.oauth.apiClient); err != nil {
			p.oauth.sendError(w, r, fmt.Errorf("get user: %w", err), 0)
			return
		}

		/** create session */
		var sessionToken SessionToken
		if sessionToken, err = p.createSession(r.Context(), tokenInfo, user); err != nil {
			p.oauth.sendError(w, r, fmt.Errorf("create session: %w", err), 0)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sessionToken)

	}
}
