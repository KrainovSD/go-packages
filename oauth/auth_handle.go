package oauth

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

func (o *Oauth) AuthHandle() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		/** Generate callback url */
		var frontendProto = getProto(r, o.frontend.Protocol)
		var frontendHost = getHost(r, o.frontend.Host)
		var callbackUrl = frontendProto + "://" + frontendHost + o.routing.CallbackPath

		/** Generate comeback url */
		var comebackPath = r.URL.Query().Get("comeback_path")
		var comebackUrl = r.URL.Query().Get("comeback_url")
		comebackPath = strings.Replace(comebackPath, frontendProto+"://", "", 1)
		comebackPath = strings.Replace(comebackPath, frontendHost, "", 1)
		if comebackUrl == "" {
			comebackUrl = frontendProto + "://" + frontendHost + comebackPath
		}

		/** Generate oauth specific variables and store them by timeKey or flowId */
		var oauthState oauthState
		var timeKey string
		if oauthState, timeKey, err = newOauthState(oauthStateOptions{
			CallbackUrl: callbackUrl,
			ComebackUrl: comebackUrl,
		}); err != nil {
			o.redirectError(redirectErrorOptions{
				w:   w,
				r:   r,
				err: fmt.Errorf("generate service state: %w", err),
			})
			return
		}
		if err = o.setOauthState(r.Context(), oauthState, timeKey); err != nil {
			o.redirectError(redirectErrorOptions{
				w:   w,
				r:   r,
				err: fmt.Errorf("set flow state: %w", err),
			})
			return
		}
		/** Set service cookies */
		http.SetCookie(w, &http.Cookie{
			Name:     o.cookieTimeKey.Name,
			Value:    timeKey,
			Path:     o.cookieTimeKey.Prefix,
			MaxAge:   o.settings.ServiceDataExpiresIn,
			HttpOnly: true,
			Secure:   frontendProto == "https",
		})
		var provider = *o.provider
		provider.RedirectURL = callbackUrl
		var opts = []oauth2.AuthCodeOption{
			oauth2.S256ChallengeOption(oauthState.CodeVerifier),
			oidc.Nonce(oauthState.Nonce),
		}
		http.Redirect(w, r, provider.AuthCodeURL(oauthState.State, opts...), http.StatusTemporaryRedirect)
	}

}
