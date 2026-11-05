package oauth

import (
	"net/http"
)

func (o *Oauth) ClearHandle() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var timeKeyCookie *http.Cookie
		var frontendProto = getProto(r, o.frontend.Protocol)
		var frontendHost = getHost(r, o.frontend.Host)
		if timeKeyCookie, err = r.Cookie(o.cookieTimeKey.Name); err == nil {
			var state logoutState
			if state, err = o.getLogoutState(r.Context(), timeKeyCookie.Value); err == nil {
				frontendHost = state.Host
				frontendProto = state.Proto
			}
		}
		var comebackUrl = frontendProto + "://" + frontendHost + o.frontend.ClearPath
		o.clearCookies(w, frontendHost)
		http.Redirect(w, r, comebackUrl, http.StatusTemporaryRedirect)
	}
}
