package oauth

type Cookie struct {
	Prefix string
	Name   string
}

type User struct {
	ID       string `json:"id"`
	UserName string `json:"username"`
	Name     string `json:"name"`
	Email    string `json:"email"`
}

type TokenInfo struct {
	AccessToken           string
	IdToken               string
	ExpiresIn             int
	RefreshToken          string
	RefreshTokenExpiresIn int
}

type SessionToken struct {
	Token   string `json:"token"`
	Expires int    `json:"expires"`
}

type IdTokenClaim struct {
	Exp int `json:"exp"`
	Iat int `json:"iat"`
}

type OidcConfig struct {
	Issuer     string `json:"issuer"`
	UserPath   string `json:"userinfo_endpoint"`
	TokenPath  string `json:"token_endpoint"`
	LoginPath  string `json:"authorization_endpoint"`
	LogoutPath string `json:"end_session_endpoint"`
}

type OidcToken struct {
	AccessToken           string `json:"access_token"`
	IdToken               string `json:"id_token"`
	ExpiresIn             int    `json:"expires_in"`
	RefreshToken          string `json:"refresh_token"`
	RefreshTokenExpiresIn int    `json:"refresh_expires_in"`
}

type RefreshTokenBody struct {
	Code         string `json:"code"`
	CodeVerifier string `json:"code_verifier"`
	State        string `json:"state"`
}

type logoutState struct {
	Host  string `json:"host"`
	Proto string `json:"proto"`
}

type flowState struct {
	State               string `json:"state"`
	Nonce               string `json:"nonce"`
	CodeVerifier        string `json:"code_verifier"`
	ClientCodeChallenge string `json:"client_code_challenge"`
	CallbackUrl         string `json:"callback_url"`
	ComebackUrl         string `json:"comeback_url"`
}

type Token = uint8

const (
	ACCESS_TOKEN Token = iota + 1
	ID_TOKEN
	REFRESH_TOKEN
)

// API
type GithubUser struct {
	ID       int    `json:"id"`
	UserName string `json:"login"`
	Name     string `json:"name"`
	Email    string `json:"email"`
}
type GitlabUser struct {
	ID       int    `json:"id"`
	UserName string `json:"username"`
	Name     string `json:"name"`
	Email    string `json:"email"`
}
type GoogleUser struct {
	ID       string `json:"id"`
	UserName string `json:"family_name"`
	Name     string `json:"name"`
	Email    string `json:"email"`
}
type YandexUser struct {
	ID       string `json:"id"`
	UserName string `json:"last_name"`
	Name     string `json:"real_name"`
	Email    string `json:"default_email"`
}

type OidcUser struct {
	ID       string `json:"sub"`
	Name     string `json:"name"`
	UserName string `json:"nickname"`
	Email    string `json:"email"`
}
