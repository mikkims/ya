package handler

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	appauth "github.com/mikkims/ya/internal/auth"
)

const (
	userCookieName = "user_id"
	userContextKey = "user_id"
)

var cookieKey = []byte("shortener-cookie-signing-key")

func (h *handler) authenticate(c *gin.Context) {
	cookie, err := c.Request.Cookie(userCookieName)
	if err == nil {
		if id, ok := verifyUserCookie(cookie.Value); ok {
			c.Set(userContextKey, id)
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
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     userCookieName,
		Value:    signUserID(id),
		Path:     "/",
		HttpOnly: true,
	})
	c.Set(userContextKey, id)
	c.Request = c.Request.WithContext(appauth.WithUserID(c.Request.Context(), id))
	c.Next()
}

func userID(c *gin.Context) string {
	id, _ := c.Get(userContextKey)
	value, _ := id.(string)
	return value
}

func newUserID() (string, error) {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

func signUserID(id string) string {
	signature := hmac.New(sha256.New, cookieKey)
	_, _ = signature.Write([]byte(id))
	value := id + "." + hex.EncodeToString(signature.Sum(nil))
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}

func verifyUserCookie(value string) (string, bool) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return "", false
	}
	id, signatureHex, ok := strings.Cut(string(decoded), ".")
	if !ok {
		return "", false
	}
	signature, err := hex.DecodeString(signatureHex)
	if err != nil {
		return "", false
	}
	expected := hmac.New(sha256.New, cookieKey)
	_, _ = expected.Write([]byte(id))
	return id, hmac.Equal(signature, expected.Sum(nil))
}
