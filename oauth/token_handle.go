package oauth

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
)

func (o *Oauth) TokenHandle() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var frontendProto = getProto(r, o.frontend.Protocol)

		var token string
		if o.cookieRefreshToken != nil {
			if token, err = o.extractToken(r, o.cookieRefreshToken); err != nil {
				o.sendError(w, r, fmt.Errorf("no token found: %w", err), 401)
				return
			}
		} else {
			if token, err = o.extractToken(r, o.cookieSessionToken); err != nil {
				o.sendError(w, r, fmt.Errorf("no token found: %w", err), 401)
				return
			}
		}

		var sessionToken SessionToken
		if o.handlers.UpdateToken != nil {
			if sessionToken, err = o.handlers.UpdateToken(r.Context(), token); err != nil {
				o.sendError(w, r, fmt.Errorf("update token: %w", err), 401)
				return
			}
		} else if o.cookieRefreshToken != nil {
			var tokenInfo TokenInfo
			if tokenInfo, err = o.exchangeTokenByRefresh(r.Context(), token); err != nil {
				o.sendError(w, r, fmt.Errorf("request token: %w", err), 401)
				return
			}
			var verifiedIdToken *oidc.IDToken
			if verifiedIdToken, err = o.verifyIdToken(r.Context(), tokenInfo.IdToken); err != nil {
				o.sendError(w, r, fmt.Errorf("validate id token: %w", err), 401)
				return
			}
			var user User
			if o.handlers.ParseUser != nil {
				if user, err = o.getUser(r.Context(), tokenInfo.AccessToken); err != nil {
					o.sendError(w, r, fmt.Errorf("get user: %w", err), 401)
					return
				}
			}
			if o.handlers.CreateSession != nil {
				if sessionToken, err = o.handlers.CreateSession(r.Context(), tokenInfo, user, verifiedIdToken); err != nil {
					o.sendError(w, r, fmt.Errorf("create session: %w", err), 401)
					return
				}
			} else if verifiedIdToken != nil {
				sessionToken = SessionToken{
					Token:   tokenInfo.IdToken,
					Expires: getIdTokenExpires(verifiedIdToken),
				}
			} else {
				sessionToken = SessionToken{
					Token:   tokenInfo.AccessToken,
					Expires: tokenInfo.ExpiresIn,
				}
			}
		} else {
			sessionToken = SessionToken{
				Token:   token,
				Expires: 0,
			}
		}
		if o.cookieSessionToken != nil {
			http.SetCookie(w, &http.Cookie{
				Name:     o.cookieSessionToken.Name,
				Value:    sessionToken.Token,
				Path:     o.cookieSessionToken.Prefix,
				MaxAge:   sessionToken.Expires,
				HttpOnly: true,
				Secure:   frontendProto == "https",
			})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sessionToken)
		return
	}
}
