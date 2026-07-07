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
		var comebackUrl string
		var clearPath = o.routing.ClearPath
		if o.handlers.EndSession != nil {
			clearPath = o.frontend.ClearRedirectPath
		}
		if comebackUrl, err = generateClearUrl(frontendProto, frontendHost, clearPath); err != nil {
			o.redirectError(redirectErrorOptions{
				w:             w,
				r:             r,
				frontendHost:  frontendHost,
				frontendProto: frontendProto,
				err:           fmt.Errorf("generate clear url: %w", err),
			})
			return
		}

		if o.handlers.EndSession == nil {
			var fallbackUrl string
			if fallbackUrl, err = generateFallbackLogoutUrl(frontendProto, frontendHost, o.routing.AuthPath, o.frontend.LogoutRedirectPath); err != nil {
				o.redirectError(redirectErrorOptions{
					w:             w,
					r:             r,
					frontendHost:  frontendHost,
					frontendProto: frontendProto,
					err:           fmt.Errorf("generate fallback url: %w", err),
				})
				return
			}
			var timeKey string
			if timeKey, err = helpers.RandomHex(32); err != nil {
				o.redirectError(redirectErrorOptions{
					w:             w,
					r:             r,
					frontendHost:  frontendHost,
					frontendProto: frontendProto,
					err:           fmt.Errorf("generate time key: %w", err),
				})
				return
			}
			if err = setLogoutState(
				r.Context(),
				logoutState{Host: frontendHost, Proto: frontendProto},
				timeKey,
				o.settings.ServiceDataExpiresIn,
				o.redis,
			); err != nil {
				o.redirectError(redirectErrorOptions{
					w:             w,
					r:             r,
					frontendHost:  frontendHost,
					frontendProto: frontendProto,
					err:           fmt.Errorf("generate time key: %w", err),
				})
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
			var tokenId string
			if tokenId, err = o.extractToken(r, o.cookieSessionToken); err != nil {
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
			if logoutUrl, err = generateLogoutUrl(o.endSessionURL, comebackUrl, tokenId, o.provider.ClientID); err != nil {
				o.redirectError(redirectErrorOptions{
					w:             w,
					r:             r,
					frontendHost:  frontendHost,
					frontendProto: frontendProto,
					err:           fmt.Errorf("generate logout url: %w", err),
				})
				return
			}
			http.Redirect(w, r, logoutUrl, http.StatusTemporaryRedirect)
		} else {
			var token string
			if token, err = o.extractToken(r, o.cookieSessionToken); err != nil {
				o.redirectError(redirectErrorOptions{
					w:             w,
					r:             r,
					frontendProto: frontendProto,
					frontendHost:  frontendHost,
					err:           fmt.Errorf("no token found: %w", err),
				})
				return
			}
			if err = o.handlers.EndSession(r.Context(), token, o.endSessionURL); err != nil {
				o.redirectError(redirectErrorOptions{
					w:             w,
					r:             r,
					frontendProto: frontendProto,
					frontendHost:  frontendHost,
					err:           fmt.Errorf("logout execute: %w", err),
				})
				return
			}
			o.clearTokens(w, frontendProto)
			http.Redirect(w, r, comebackUrl, http.StatusTemporaryRedirect)
		}
	}
}
