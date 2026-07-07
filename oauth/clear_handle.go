package oauth

import (
	"fmt"
	"net/http"
)

func (o *Oauth) ClearHandle() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var timeKeyCookie *http.Cookie
		if timeKeyCookie, err = r.Cookie(o.cookieTimeKey.Name); err != nil {
			o.redirectError(redirectErrorOptions{
				w:   w,
				r:   r,
				err: fmt.Errorf("get time key: %w", err),
			})
			return
		}
		var state logoutState
		if state, err = o.getLogoutState(r.Context(), timeKeyCookie.Value); err != nil {
			o.redirectError(redirectErrorOptions{
				w:   w,
				r:   r,
				err: fmt.Errorf("get logout state: %w", err),
			})
			return
		}
		var comebackUrl = state.Proto + "://" + state.Host + o.frontend.ClearPath
		o.clearCookies(w, state.Proto)
		http.Redirect(w, r, comebackUrl, http.StatusTemporaryRedirect)

	}
}
