package oauth

import (
	"fmt"
	"net/http"
)

func (o *Oauth) ClearHandle() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		/** extract logout state */
		var timeKey string
		if o.cookieTimeKey != nil {
			var timeKeyCookie *http.Cookie
			if timeKeyCookie, err = r.Cookie(o.cookieTimeKey.Name); err != nil {
				o.redirectError(redirectErrorOptions{
					w:   w,
					r:   r,
					err: fmt.Errorf("get time key: %w", err),
				})
				return
			}
			timeKey = timeKeyCookie.Value
		}
		var state logoutState
		if state, err = getLogoutState(r.Context(), timeKey, o.redis); err != nil {
			o.redirectError(redirectErrorOptions{
				w:   w,
				r:   r,
				err: fmt.Errorf("get logout state: %w", err),
			})
			return
		}
		var comebackUrl = state.Proto + "://" + state.Host + o.frontend.ClearRedirectPath
		if o.cookieTimeKey != nil {
			http.SetCookie(w, &http.Cookie{
				Name:     o.cookieTimeKey.Name,
				Value:    "",
				Path:     o.cookieTimeKey.Prefix,
				MaxAge:   -1,
				HttpOnly: true,
				Secure:   state.Proto == "https",
			})
		}
		if o.cookieRefreshToken != nil {
			http.SetCookie(w, &http.Cookie{
				Name:     o.cookieRefreshToken.Name,
				Value:    "",
				Path:     o.cookieRefreshToken.Prefix,
				MaxAge:   -1,
				HttpOnly: true,
				Secure:   state.Proto == "https",
			})
		}
		if o.cookieSessionToken != nil {
			http.SetCookie(w, &http.Cookie{
				Name:     o.cookieSessionToken.Name,
				Value:    "",
				Path:     o.cookieSessionToken.Prefix,
				MaxAge:   -1,
				HttpOnly: true,
				Secure:   state.Proto == "https",
			})
		}
		http.Redirect(w, r, comebackUrl, http.StatusTemporaryRedirect)

	}
}
