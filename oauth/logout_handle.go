package oauth

import (
	"fmt"
	"net/http"
)

func (p *OauthProvider) LogoutHandle() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var frontendProto = getProto(r, p.oauth.frontendProtocol)
		var frontendHost = getHost(r, p.oauth.frontendHost)
		var comebackUrl string
		if comebackUrl, err = generateClearUrl(frontendProto, frontendHost, p.clearPath); err != nil {
			p.oauth.redirectError(redirectErrorOptions{
				w:             w,
				r:             r,
				frontendHost:  frontendHost,
				frontendProto: frontendProto,
				err:           fmt.Errorf("generate clear url: %w", err),
			})
			return
		}
		var fallbackUrl string
		if fallbackUrl, err = generateFallbackLogoutUrl(frontendProto, frontendHost, p.startAuthPath, p.oauth.frontendLogoutPath); err != nil {
			p.oauth.redirectError(redirectErrorOptions{
				w:             w,
				r:             r,
				frontendHost:  frontendHost,
				frontendProto: frontendProto,
				err:           fmt.Errorf("generate fallback url: %w", err),
			})
			return
		}

		var timeKey string
		if timeKey, err = randomHex(32); err != nil {
			p.oauth.redirectError(redirectErrorOptions{
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
			p.oauth.serviceDataExpires,
			p.oauth.redis,
		); err != nil {
			p.oauth.redirectError(redirectErrorOptions{
				w:             w,
				r:             r,
				frontendHost:  frontendHost,
				frontendProto: frontendProto,
				err:           fmt.Errorf("generate time key: %w", err),
			})
			return
		}
		if p.oauth.cookieTimeKey != nil {
			http.SetCookie(w, &http.Cookie{
				Name:     p.oauth.cookieTimeKey.Name,
				Value:    timeKey,
				Path:     p.oauth.cookieTimeKey.Prefix,
				MaxAge:   p.oauth.serviceDataExpires,
				HttpOnly: true,
				Secure:   frontendProto == "https",
			})
		}
		var tokenId string
		if tokenId, err = p.oauth.extractToken(r, p.oauth.cookieSessionToken); err != nil {
			// use fallback url for re-auth and set token id to cookie
			p.oauth.redirectError(redirectErrorOptions{
				w:           w,
				r:           r,
				comebackUrl: fallbackUrl,
				err:         fmt.Errorf("tokenId not found: %w", err),
			})
			return

		}
		var logoutUrl string
		if logoutUrl, err = generateLogoutUrl(p.logoutPath, comebackUrl, tokenId, p.clientId); err != nil {
			p.oauth.redirectError(redirectErrorOptions{
				w:             w,
				r:             r,
				frontendHost:  frontendHost,
				frontendProto: frontendProto,
				err:           fmt.Errorf("generate logout url: %w", err),
			})
			return
		}

		http.Redirect(w, r, logoutUrl, http.StatusTemporaryRedirect)

	}
}

func (o *Oauth) LogoutHandle() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var frontendProto = getProto(r, o.frontendProtocol)
		var frontendHost = getHost(r, o.frontendHost)
		var comebackUrl string
		if comebackUrl, err = generateClearUrl(frontendProto, frontendHost, o.frontendClearPath); err != nil {
			o.redirectError(redirectErrorOptions{
				w:             w,
				r:             r,
				frontendProto: frontendProto,
				frontendHost:  frontendHost,
				err:           fmt.Errorf("generate clear url: %w", err),
			})
			return
		}
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
		if o.logout != nil {
			if err = o.logout(token); err != nil {
				o.redirectError(redirectErrorOptions{
					w:             w,
					r:             r,
					frontendProto: frontendProto,
					frontendHost:  frontendHost,
					err:           fmt.Errorf("logout execute: %w", err),
				})
				return
			}
		}
		if o.cookieTimeKey != nil {
			http.SetCookie(w, &http.Cookie{
				Name:     o.cookieTimeKey.Name,
				Value:    "",
				Path:     o.cookieTimeKey.Prefix,
				MaxAge:   -1,
				HttpOnly: true,
				Secure:   frontendProto == "https",
			})
		}
		if o.cookieRefreshToken != nil {
			http.SetCookie(w, &http.Cookie{
				Name:     o.cookieRefreshToken.Name,
				Value:    "",
				Path:     o.cookieRefreshToken.Prefix,
				MaxAge:   -1,
				HttpOnly: true,
				Secure:   frontendProto == "https",
			})
		}
		if o.cookieSessionToken != nil {
			http.SetCookie(w, &http.Cookie{
				Name:     o.cookieSessionToken.Name,
				Value:    "",
				Path:     o.cookieSessionToken.Prefix,
				MaxAge:   -1,
				HttpOnly: true,
				Secure:   frontendProto == "https",
			})
		}
		http.Redirect(w, r, comebackUrl, http.StatusTemporaryRedirect)
	}
}
