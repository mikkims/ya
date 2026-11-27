package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
	appauth "github.com/mikkims/ya/internal/auth"
)

const (
	userCookieName = "user_id"
)

func (h *handler) authenticate(c *gin.Context) {
	cookie, err := c.Request.Cookie(userCookieName)
	if err == nil {
		if id, ok := appauth.UserIDFromToken(cookie.Value); ok {
			c.Request = c.Request.WithContext(appauth.WithUserID(c.Request.Context(), id))
			c.Next()
			return
		}
	}

	id, err := newUserID()
	if err != nil {
		internalServerError(c)
		c.Abort()
		return
	}
	token, err := appauth.NewToken(id)
	if err != nil {
		internalServerError(c)
		c.Abort()
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     userCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
	})
	c.Request = c.Request.WithContext(appauth.WithUserID(c.Request.Context(), id))
	c.Next()
}

func newUserID() (string, error) {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}
