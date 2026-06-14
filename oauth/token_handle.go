package oauth

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func (p *OauthProvider) TokenHandle() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var frontendProto = getProto(r, p.oauth.frontendProtocol)

		var token string
		if p.oauth.cookieRefreshToken != nil {
			if token, err = p.oauth.extractToken(r, p.oauth.cookieRefreshToken); err != nil {
				p.oauth.sendError(w, r, fmt.Errorf("no token found: %w", err), 401)
				return
			}
		} else {
			if token, err = p.oauth.extractToken(r, p.oauth.cookieSessionToken); err != nil {
				p.oauth.sendError(w, r, fmt.Errorf("no token found: %w", err), 401)
				return
			}
		}

		var sessionToken SessionToken
		if p.oauth.updateToken != nil {
			if sessionToken, err = p.oauth.updateToken(r.Context(), token); err != nil {
				p.oauth.sendError(w, r, fmt.Errorf("update token: %w", err), 401)
				return
			}
		} else if p.oauth.cookieRefreshToken != nil {
			var tokenInfo TokenInfo
			if tokenInfo, err = p.getTokenByRefresh(r.Context(), p.oauth.apiClient, token); err != nil {
				p.oauth.sendError(w, r, fmt.Errorf("request token: %w", err), 401)
				return
			}
			/** validate if OIDC protocol supported  */
			if err = p.oidcValidate(tokenInfo.IdToken); err != nil {
				p.oauth.sendError(w, r, fmt.Errorf("validate id token: %w", err), 401)
				return
			}
			/** get user */
			var user User
			if p.parseUser != nil {
				if user, err = p.getUser(r.Context(), tokenInfo.AccessToken, p.oauth.apiClient); err != nil {
					p.oauth.sendError(w, r, fmt.Errorf("get user: %w", err), 401)
					return
				}
			}
			if p.createSession != nil {
				if sessionToken, err = p.createSession(r.Context(), tokenInfo, user); err != nil {
					p.oauth.sendError(w, r, fmt.Errorf("create session: %w", err), 401)
					return
				}
			} else if tokenInfo.IdToken != "" {
				sessionToken = SessionToken{
					Token:   tokenInfo.IdToken,
					Expires: getIdTokenExpires(tokenInfo.IdToken, tokenInfo.ExpiresIn),
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

		if p.oauth.cookieSessionToken != nil {
			http.SetCookie(w, &http.Cookie{
				Name:     p.oauth.cookieSessionToken.Name,
				Value:    sessionToken.Token,
				Path:     p.oauth.cookieSessionToken.Prefix,
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

func (o *Oauth) TokenHandle() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var frontendProto = getProto(r, o.frontendProtocol)

		var token string
		if token, err = o.extractToken(r, o.cookieSessionToken); err != nil {
			o.sendError(w, r, fmt.Errorf("no token found: %w", err), 401)
			return
		}
		var sessionToken SessionToken
		if sessionToken, err = o.updateToken(r.Context(), token); err != nil {
			o.sendError(w, r, fmt.Errorf("update token: %w", err), 401)
			return
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
	}
}
