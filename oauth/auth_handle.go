package oauth

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

func (p *OauthProvider) AuthHandle() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		/** Generate callback url */
		var frontendProto = getProto(r, p.oauth.frontendProtocol)
		var frontendHost = getHost(r, p.oauth.frontendHost)
		var callbackUrl = frontendProto + "://" + frontendHost + p.callbackPath

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
			p.oauth.redirectError(redirectErrorOptions{
				w:   w,
				r:   r,
				err: fmt.Errorf("generate service state: %w", err),
			})
			return
		}
		if err = p.oauth.setOauthState(r.Context(), oauthState, timeKey); err != nil {
			p.oauth.redirectError(redirectErrorOptions{
				w:   w,
				r:   r,
				err: fmt.Errorf("set flow state: %w", err),
			})
			return
		}
		/** Set service cookies */
		http.SetCookie(w, &http.Cookie{
			Name:     p.oauth.cookieTimeKey.Name,
			Value:    timeKey,
			Path:     p.oauth.cookieTimeKey.Prefix,
			MaxAge:   p.oauth.serviceDataExpires,
			HttpOnly: true,
			Secure:   frontendProto == "https",
		})
		var config = *p.config
		config.RedirectURL = callbackUrl
		var opts = []oauth2.AuthCodeOption{
			oauth2.S256ChallengeOption(oauthState.CodeVerifier),
			oidc.Nonce(oauthState.Nonce),
		}
		http.Redirect(w, r, config.AuthCodeURL(oauthState.State, opts...), http.StatusTemporaryRedirect)
	}

}
