package oauth

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/coreos/go-oidc/v3/oidc"
)

func (o *Oauth) CallbackHandle() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var code = r.URL.Query().Get("code")
		var state = r.URL.Query().Get("state")
		var timeKeyCookie *http.Cookie
		if timeKeyCookie, err = r.Cookie(o.cookieTimeKey.Name); err != nil {
			o.redirectError(redirectErrorOptions{
				w:   w,
				r:   r,
				err: fmt.Errorf("get time key: %w", err),
			})
			return
		}
		var oauthState oauthState
		if oauthState, err = o.getOauthState(r.Context(), timeKeyCookie.Value); err != nil {
			o.redirectError(redirectErrorOptions{
				w:   w,
				r:   r,
				err: fmt.Errorf("get flow state: %w", err),
			})
			return
		}
		var comebackUrl *url.URL
		if comebackUrl, err = url.Parse(oauthState.ComebackUrl); err != nil {
			o.redirectError(redirectErrorOptions{
				w:   w,
				r:   r,
				err: fmt.Errorf("get comeback url: %w", err),
			})
			return
		}
		var proto = comebackUrl.Scheme
		var host = comebackUrl.Host
		http.SetCookie(w, &http.Cookie{
			Name:     o.cookieTimeKey.Name,
			Value:    "",
			Path:     o.cookieTimeKey.Prefix,
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   proto == "https",
		})
		if !safeCompare(oauthState.State, state) {
			o.redirectError(redirectErrorOptions{
				w:             w,
				r:             r,
				frontendHost:  host,
				frontendProto: proto,
				err:           fmt.Errorf("the state is not the same"),
			})
			return
		}
		var tokenInfo TokenInfo
		if tokenInfo, err = o.exchangeToken(r.Context(), code, oauthState.CodeVerifier, oauthState.CallbackUrl); err != nil {
			o.redirectError(redirectErrorOptions{
				w:             w,
				r:             r,
				frontendHost:  host,
				frontendProto: proto,
				err:           fmt.Errorf("get token: %w", err),
			})
			return
		}
		var verifiedIdToken *oidc.IDToken
		if verifiedIdToken, err = o.verifyIdToken(r.Context(), tokenInfo.IdToken); err != nil {
			o.redirectError(redirectErrorOptions{
				w:             w,
				r:             r,
				frontendHost:  host,
				frontendProto: proto,
				err:           fmt.Errorf("validate oidc: %w", err),
			})
			return
		}
		if verifiedIdToken != nil && !safeCompare(verifiedIdToken.Nonce, oauthState.Nonce) {
			o.redirectError(redirectErrorOptions{
				w:             w,
				r:             r,
				frontendHost:  host,
				frontendProto: proto,
				err:           fmt.Errorf("validate nonce, expected: %s, received: %s", oauthState.Nonce, verifiedIdToken.Nonce),
			})
			return
		}
		var user User
		if o.handlers.ParseUser != nil {
			if user, err = o.getUser(r.Context(), tokenInfo.AccessToken); err != nil {
				o.redirectError(redirectErrorOptions{
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
		if o.handlers.CreateSession != nil {
			if sessionToken, err = o.handlers.CreateSession(r.Context(), tokenInfo, user, verifiedIdToken); err != nil {
				o.redirectError(redirectErrorOptions{
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

		if o.cookieRefreshToken != nil {
			if tokenInfo.RefreshToken != "" && tokenInfo.RefreshTokenExpiresIn != 0 {
				http.SetCookie(w, &http.Cookie{
					Name:     o.cookieRefreshToken.Name,
					Value:    tokenInfo.RefreshToken,
					Path:     o.cookieRefreshToken.Prefix,
					MaxAge:   tokenInfo.RefreshTokenExpiresIn,
					HttpOnly: true,
					Secure:   proto == "https",
				})
			} else {
				o.log.Warn("not found refresh token or expires in", "refreshToken", tokenInfo.RefreshToken == "", "expiresIn", tokenInfo.RefreshTokenExpiresIn == 0)
			}
		}

		if o.cookieSessionToken != nil {
			http.SetCookie(w, &http.Cookie{
				Name:     o.cookieSessionToken.Name,
				Value:    sessionToken.Token,
				Path:     o.cookieSessionToken.Prefix,
				MaxAge:   sessionToken.Expires,
				HttpOnly: true,
				Secure:   proto == "https",
			})
		}

		comebackQuery := comebackUrl.Query()
		comebackQuery.Set(o.frontend.ExpiresQuery, strconv.Itoa(sessionToken.Expires))
		comebackUrl.RawQuery = comebackQuery.Encode()
		http.Redirect(w, r, comebackUrl.String(), http.StatusTemporaryRedirect)

	}
}
