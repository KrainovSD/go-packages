package oauth

import (
	"fmt"
	"net/http"
)

func (p *OauthProvider) ClearHandle() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		/** extract logout state */
		var timeKey string
		if p.oauth.cookieTimeKey != nil {
			var timeKeyCookie *http.Cookie
			if timeKeyCookie, err = r.Cookie(p.oauth.cookieTimeKey.Name); err != nil {
				p.oauth.redirectError(redirectErrorOptions{
					w:   w,
					r:   r,
					err: fmt.Errorf("get time key: %w", err),
				})
				return
			}
			timeKey = timeKeyCookie.Value
		}
		var state logoutState
		if state, err = getLogoutState(r.Context(), timeKey, p.oauth.redis); err != nil {
			p.oauth.redirectError(redirectErrorOptions{
				w:   w,
				r:   r,
				err: fmt.Errorf("get logout state: %w", err),
			})
			return
		}
		var comebackUrl = state.Proto + "://" + state.Host + p.oauth.frontendClearPath

		if p.oauth.cookieTimeKey != nil {
			http.SetCookie(w, &http.Cookie{
				Name:     p.oauth.cookieTimeKey.Name,
				Value:    "",
				Path:     p.oauth.cookieTimeKey.Prefix,
				MaxAge:   -1,
				HttpOnly: true,
				Secure:   state.Proto == "https",
			})
		}
		if p.oauth.cookieRefreshToken != nil {
			http.SetCookie(w, &http.Cookie{
				Name:     p.oauth.cookieRefreshToken.Name,
				Value:    "",
				Path:     p.oauth.cookieRefreshToken.Prefix,
				MaxAge:   -1,
				HttpOnly: true,
				Secure:   state.Proto == "https",
			})
		}
		if p.oauth.cookieSessionToken != nil {
			http.SetCookie(w, &http.Cookie{
				Name:     p.oauth.cookieSessionToken.Name,
				Value:    "",
				Path:     p.oauth.cookieSessionToken.Prefix,
				MaxAge:   -1,
				HttpOnly: true,
				Secure:   state.Proto == "https",
			})
		}
		http.Redirect(w, r, comebackUrl, http.StatusTemporaryRedirect)

	}
}
