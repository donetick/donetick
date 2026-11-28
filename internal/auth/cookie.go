package auth

import "github.com/gin-gonic/gin"

// SetAuthCookie writes an auth cookie (refresh_token / access_token) as an
// HttpOnly cookie on path "/". The cookie is marked Secure unless
// server.insecure_cookies is enabled.
func (s *TokenService) SetAuthCookie(c *gin.Context, name, value string, maxAge int) {
	c.SetCookie(name, value, maxAge, "/", "", !s.insecureCookies, true)
}
