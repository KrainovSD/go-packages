package oauth

import (
	"fmt"
	"io"
	"net/http"
)

func (p *OauthProvider) ProxyHandle() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		var flowId = r.Header.Get("x-flow-id")
		if flowId == "" {
			p.oauth.sendError(w, r, fmt.Errorf("empty flow id"), 0)
			return
		}

		var clientCodeChallenge = r.URL.Query().Get("code_challenge")

		if clientCodeChallenge == "" {
			p.oauth.sendError(w, r, fmt.Errorf("empty code challenge"), 0)
			return
		}
		var callbackUrl = r.URL.Query().Get("redirect_uri")
		if callbackUrl == "" {
			p.oauth.sendError(w, r, fmt.Errorf("empty redirect url"), 0)
			return
		}

		/** Generate oauth specific variables and store them by timeKey or flowId */
		var ostate oauthState
		if ostate, err = generateOauthServiceState(); err != nil {
			p.oauth.sendError(w, r, fmt.Errorf("generate service state: %w", err), 0)
			return
		}
		fsate := flowState{
			State:               ostate.State,
			Nonce:               ostate.Nonce,
			CodeVerifier:        ostate.CodeVerifier,
			ClientCodeChallenge: clientCodeChallenge,
			CallbackUrl:         callbackUrl,
		}
		if err = setFlowState(r.Context(), fsate, flowId, p.oauth.serviceDataExpires, p.oauth.redis); err != nil {
			p.oauth.sendError(w, r, fmt.Errorf("set flow state: %w", err), 0)
			return
		}

		/** Generate oauth url */
		var logUrl string
		if logUrl, err = generateLogUrl(authUrlOptions{
			Url:           p.loginPath,
			Nonce:         ostate.Nonce,
			CodeChallenge: ostate.CodeChallenge,
			State:         ostate.State,
			ClientId:      p.clientId,
			CallbackUrl:   callbackUrl,
			Scopes:        p.scopes,
		}); err != nil {
			p.oauth.sendError(w, r, fmt.Errorf("generate logUrl: %w", err), 0)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		io.WriteString(w, logUrl)

	}

}
