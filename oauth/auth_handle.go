package oauth

import (
	"fmt"
	"net/http"
	"strings"
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
		var ostate oauthState
		if ostate, err = generateOauthServiceState(); err != nil {
			p.oauth.redirectError(redirectErrorOptions{
				w:   w,
				r:   r,
				err: fmt.Errorf("generate service state: %w", err),
			})
			return
		}
		fsate := flowState{
			State:        ostate.State,
			Nonce:        ostate.Nonce,
			CodeVerifier: ostate.CodeVerifier,
			CallbackUrl:  callbackUrl,
			ComebackUrl:  comebackUrl,
		}
		if err = setFlowState(r.Context(), fsate, ostate.TimeKey, p.oauth.serviceDataExpires, p.oauth.redis); err != nil {
			p.oauth.redirectError(redirectErrorOptions{
				w:   w,
				r:   r,
				err: fmt.Errorf("set flow state: %w", err),
			})
			return
		}

		/** Set service cookies */
		if p.oauth.cookieTimeKey != nil {
			http.SetCookie(w, &http.Cookie{
				Name:     p.oauth.cookieTimeKey.Name,
				Value:    ostate.TimeKey,
				Path:     p.oauth.cookieTimeKey.Prefix,
				MaxAge:   p.oauth.serviceDataExpires,
				HttpOnly: true,
				Secure:   frontendProto == "https",
			})
		}

		/** Generate oauth url */
		var logUrl string
		if logUrl, err = generateLogUrl(authUrlOptions{
			Url:           p.loginPath,
			Nonce:         ostate.Nonce,
			State:         ostate.State,
			ClientId:      p.clientId,
			CallbackUrl:   callbackUrl,
			Scopes:        p.scopes,
			CodeChallenge: ostate.CodeChallenge,
		}); err != nil {
			p.oauth.redirectError(redirectErrorOptions{
				w:   w,
				r:   r,
				err: fmt.Errorf("generate logUrl: %w", err),
			})
			return
		}

		http.Redirect(w, r, logUrl, http.StatusTemporaryRedirect)

	}

}
