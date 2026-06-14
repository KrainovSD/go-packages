package oauth

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

func (p *OauthProvider) CallbackHandle() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var code = r.URL.Query().Get("code")
		var state = r.URL.Query().Get("state")

		/** extract flow state */
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

		var fstate flowState
		if fstate, err = getFlowState(r.Context(), timeKey, p.oauth.redis); err != nil {
			p.oauth.redirectError(redirectErrorOptions{
				w:   w,
				r:   r,
				err: fmt.Errorf("get flow state: %w", err),
			})
			return
		}

		/** get comeback url */
		var comebackUrl *url.URL
		if comebackUrl, err = url.Parse(fstate.ComebackUrl); err != nil {
			p.oauth.redirectError(redirectErrorOptions{
				w:   w,
				r:   r,
				err: fmt.Errorf("get comeback url: %w", err),
			})
			return
		}
		var proto = comebackUrl.Scheme
		var host = comebackUrl.Host

		/** clear service cookies */
		if p.oauth.cookieTimeKey != nil {
			http.SetCookie(w, &http.Cookie{
				Name:     p.oauth.cookieTimeKey.Name,
				Value:    "",
				Path:     p.oauth.cookieTimeKey.Prefix,
				MaxAge:   -1,
				HttpOnly: true,
				Secure:   proto == "https",
			})
		}
		/** check state */
		if !safeCompare(fstate.State, state) {
			p.oauth.redirectError(redirectErrorOptions{
				w:             w,
				r:             r,
				frontendHost:  host,
				frontendProto: proto,
				err:           fmt.Errorf("the state is not the same"),
			})
			return
		}
		/** get access token */
		var tokenInfo TokenInfo
		if tokenInfo, err = p.getToken(r.Context(), getTokenOptions{
			CodeVerifier: fstate.CodeVerifier,
			Code:         code,
			CallbackUrl:  fstate.CallbackUrl,
			ApiClient:    p.oauth.apiClient,
		}); err != nil {
			p.oauth.redirectError(redirectErrorOptions{
				w:             w,
				r:             r,
				frontendHost:  host,
				frontendProto: proto,
				err:           fmt.Errorf("get token: %w", err),
			})
			return
		}
		/** validate if OIDC protocol supported  */
		if err = p.oidcFlowValidate(tokenInfo.IdToken, fstate.Nonce); err != nil {
			p.oauth.redirectError(redirectErrorOptions{
				w:             w,
				r:             r,
				frontendHost:  host,
				frontendProto: proto,
				err:           fmt.Errorf("validate oidc: %w", err),
			})
			return
		}
		/** get user */
		var user User
		if p.parseUser != nil {
			if user, err = p.getUser(r.Context(), tokenInfo.AccessToken, p.oauth.apiClient); err != nil {
				p.oauth.redirectError(redirectErrorOptions{
					w:             w,
					r:             r,
					frontendHost:  host,
					frontendProto: proto,
					err:           fmt.Errorf("get user: %w", err),
				})
				return
			}
		}

		/** create session */
		var sessionToken SessionToken
		if p.createSession != nil {
			if sessionToken, err = p.createSession(r.Context(), tokenInfo, user); err != nil {
				p.oauth.redirectError(redirectErrorOptions{
					w:             w,
					r:             r,
					frontendHost:  host,
					frontendProto: proto,
					err:           fmt.Errorf("create session: %w", err),
				})
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

		if p.oauth.cookieRefreshToken != nil && tokenInfo.RefreshToken != "" && tokenInfo.RefreshTokenExpiresIn != 0 {
			http.SetCookie(w, &http.Cookie{
				Name:     p.oauth.cookieRefreshToken.Name,
				Value:    tokenInfo.RefreshToken,
				Path:     p.oauth.cookieRefreshToken.Prefix,
				MaxAge:   tokenInfo.RefreshTokenExpiresIn,
				HttpOnly: true,
				Secure:   proto == "https",
			})
		}
		if p.oauth.cookieSessionToken != nil {
			http.SetCookie(w, &http.Cookie{
				Name:     p.oauth.cookieSessionToken.Name,
				Value:    sessionToken.Token,
				Path:     p.oauth.cookieSessionToken.Prefix,
				MaxAge:   sessionToken.Expires,
				HttpOnly: true,
				Secure:   proto == "https",
			})
		}

		comebackQuery := comebackUrl.Query()
		comebackQuery.Set(p.oauth.queryExpires, strconv.Itoa(sessionToken.Expires))
		comebackUrl.RawQuery = comebackQuery.Encode()
		http.Redirect(w, r, comebackUrl.String(), http.StatusTemporaryRedirect)

	}
}
