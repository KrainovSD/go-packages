package oauth

import (
	"fmt"
	"io"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

func (o *Oauth) AuthProxyHandle() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var flowId = r.Header.Get("x-flow-id")
		if flowId == "" {
			o.sendError(w, r, fmt.Errorf("empty flow id"), 0)
			return
		}
		var clientCodeChallenge = r.URL.Query().Get("code_challenge")
		if clientCodeChallenge == "" {
			o.sendError(w, r, fmt.Errorf("empty code challenge"), 0)
			return
		}
		var callbackUrl = r.URL.Query().Get("redirect_uri")
		if callbackUrl == "" {
			o.sendError(w, r, fmt.Errorf("empty redirect url"), 0)
			return
		}
		var oauthState oauthState
		if oauthState, _, err = newOauthState(oauthStateOptions{
			CallbackUrl:         callbackUrl,
			ClientCodeChallenge: clientCodeChallenge,
		}); err != nil {
			o.sendError(w, r, fmt.Errorf("generate service state: %w", err), 0)
			return
		}
		if err = o.setOauthState(r.Context(), oauthState, flowId); err != nil {
			o.sendError(w, r, fmt.Errorf("set flow state: %w", err), 0)
			return
		}
		var provider = *o.provider
		provider.RedirectURL = callbackUrl
		var opts = []oauth2.AuthCodeOption{
			oauth2.S256ChallengeOption(oauthState.CodeVerifier),
			oidc.Nonce(oauthState.Nonce),
		}
		w.Header().Set("Content-Type", "text/plain")
		io.WriteString(w, provider.AuthCodeURL(oauthState.State, opts...))
	}
}
