package oauth

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/coreos/go-oidc/v3/oidc"
)

func (p *OauthProvider) CallbackHandle() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var code = r.URL.Query().Get("code")
		var state = r.URL.Query().Get("state")
		var timeKeyCookie *http.Cookie
		if timeKeyCookie, err = r.Cookie(p.oauth.cookieTimeKey.Name); err != nil {
			p.oauth.redirectError(redirectErrorOptions{
				w:   w,
				r:   r,
				err: fmt.Errorf("get time key: %w", err),
			})
			return
		}
		var oauthState oauthState
		if oauthState, err = p.oauth.getOauthState(r.Context(), timeKeyCookie.Value); err != nil {
			p.oauth.redirectError(redirectErrorOptions{
				w:   w,
				r:   r,
				err: fmt.Errorf("get flow state: %w", err),
			})
			return
		}
		var comebackUrl *url.URL
		if comebackUrl, err = url.Parse(oauthState.ComebackUrl); err != nil {
			p.oauth.redirectError(redirectErrorOptions{
				w:   w,
				r:   r,
				err: fmt.Errorf("get comeback url: %w", err),
			})
			return
		}
		var proto = comebackUrl.Scheme
		var host = comebackUrl.Host
		http.SetCookie(w, &http.Cookie{
			Name:     p.oauth.cookieTimeKey.Name,
			Value:    "",
			Path:     p.oauth.cookieTimeKey.Prefix,
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   proto == "https",
		})
		if !safeCompare(oauthState.State, state) {
			p.oauth.redirectError(redirectErrorOptions{
				w:             w,
				r:             r,
				frontendHost:  host,
				frontendProto: proto,
				err:           fmt.Errorf("the state is not the same"),
			})
			return
		}
		var tokenInfo TokenInfo
		if tokenInfo, err = p.exchangeToken(r.Context(), code, oauthState.CodeVerifier, oauthState.CallbackUrl); err != nil {
			p.oauth.redirectError(redirectErrorOptions{
				w:             w,
				r:             r,
				frontendHost:  host,
				frontendProto: proto,
				err:           fmt.Errorf("get token: %w", err),
			})
			return
		}
		var verifiedIdToken *oidc.IDToken
		if verifiedIdToken, err = p.verifyIdToken(r.Context(), tokenInfo.IdToken); err != nil {
			p.oauth.redirectError(redirectErrorOptions{
				w:             w,
				r:             r,
				frontendHost:  host,
				frontendProto: proto,
				err:           fmt.Errorf("validate oidc: %w", err),
			})
			return
		}
		if verifiedIdToken != nil && !safeCompare(verifiedIdToken.Nonce, oauthState.Nonce) {
			p.oauth.redirectError(redirectErrorOptions{
				w:             w,
				r:             r,
				frontendHost:  host,
				frontendProto: proto,
				err:           fmt.Errorf("validate nonce, expected: %s, received: %s", oauthState.Nonce, verifiedIdToken.Nonce),
			})
			return
		}
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
		var sessionToken SessionToken
		if p.createSession != nil {
			if sessionToken, err = p.createSession(r.Context(), tokenInfo, user, verifiedIdToken); err != nil {
				p.oauth.redirectError(redirectErrorOptions{
					w:             w,
					r:             r,
					frontendHost:  host,
					frontendProto: proto,
					err:           fmt.Errorf("create session: %w", err),
				})
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

		if p.oauth.cookieRefreshToken != nil {
			if tokenInfo.RefreshToken != "" && tokenInfo.RefreshTokenExpiresIn != 0 {
				http.SetCookie(w, &http.Cookie{
					Name:     p.oauth.cookieRefreshToken.Name,
					Value:    tokenInfo.RefreshToken,
					Path:     p.oauth.cookieRefreshToken.Prefix,
					MaxAge:   tokenInfo.RefreshTokenExpiresIn,
					HttpOnly: true,
					Secure:   proto == "https",
				})
			} else {
				p.oauth.log.Warn("not found refresh token or expires in", "refreshToken", tokenInfo.RefreshToken == "", "expiresIn", tokenInfo.RefreshTokenExpiresIn == 0)
			}
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
