package oauth

import (
	"fmt"
	"net/http"

	"github.com/KrainovSD/go-packages/helpers"
)

func (o *Oauth) EndSessionHandle() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var frontendProto = getProto(r, o.frontend.Protocol)
		var frontendHost = getHost(r, o.frontend.Host)
		var redirectWithError = func(err error) {
			o.redirectError(redirectErrorOptions{
				w:             w,
				r:             r,
				frontendHost:  frontendHost,
				frontendProto: frontendProto,
				err:           err,
			})
		}
		if o.endSessionURL == "" {
			var comebackUrl string
			if comebackUrl, err = generateClearUrl(frontendProto, frontendHost, o.frontend.ClearPath); err != nil {
				redirectWithError(fmt.Errorf("new clear url: %w", err))
				return
			}
			o.clearCookies(w, frontendProto)
			http.Redirect(w, r, comebackUrl, http.StatusTemporaryRedirect)
			return
		}

		if o.handlers.EndSession == nil {
			var comebackUrl string
			if comebackUrl, err = generateClearUrl(frontendProto, frontendHost, o.routing.ClearPath); err != nil {
				redirectWithError(fmt.Errorf("new clear url: %w", err))
				return
			}
			var fallbackUrl string
			if fallbackUrl, err = generateFallbackLogoutUrl(frontendProto, frontendHost, o.routing.AuthPath, o.frontend.LogoutPath); err != nil {
				redirectWithError(fmt.Errorf("new fallback url: %w", err))
				return
			}
			var timeKey string
			if timeKey, err = helpers.RandomHex(32); err != nil {
				redirectWithError(fmt.Errorf("new time key: %w", err))
				return
			}
			if err = o.setLogoutState(r.Context(), logoutState{Host: frontendHost, Proto: frontendProto}, timeKey); err != nil {
				redirectWithError(fmt.Errorf("set logout state: %w", err))
				return
			}
			http.SetCookie(w, &http.Cookie{
				Name:     o.cookieTimeKey.Name,
				Value:    timeKey,
				Path:     o.cookieTimeKey.Prefix,
				MaxAge:   o.settings.ServiceDataExpiresIn,
				HttpOnly: true,
				Secure:   frontendProto == "https",
			})
			var token string
			if token, err = o.extractToken(r, o.cookieSessionToken); err != nil {
				// use fallback url for re-auth and set token id to cookie
				o.redirectError(redirectErrorOptions{
					w:           w,
					r:           r,
					comebackUrl: fallbackUrl,
					err:         fmt.Errorf("tokenId not found: %w", err),
				})
				return
			}
			var logoutUrl string
			if logoutUrl, err = generateLogoutUrl(o.endSessionURL, comebackUrl, token, o.provider.ClientID); err != nil {
				redirectWithError(fmt.Errorf("new logout url: %w", err))
				return
			}
			http.Redirect(w, r, logoutUrl, http.StatusTemporaryRedirect)
		} else {
			var comebackUrl string
			if comebackUrl, err = generateClearUrl(frontendProto, frontendHost, o.frontend.ClearPath); err != nil {
				redirectWithError(fmt.Errorf("new clear url: %w", err))
				return
			}
			var token, _ = o.extractToken(r, o.cookieSessionToken)
			if err = o.handlers.EndSession(r.Context(), token, o.endSessionURL); err != nil {
				redirectWithError(fmt.Errorf("process end session: %w", err))
			}
			o.clearCookies(w, frontendProto)
			http.Redirect(w, r, comebackUrl, http.StatusTemporaryRedirect)
		}
	}
}
