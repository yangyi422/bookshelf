package auth

import (
	"net/http"
	"time"

	"bookshelf/internal/database"
	"bookshelf/internal/proxy"
	"github.com/gin-gonic/gin"
)

const CookieName = "bookshelf_session"
const userKey = "current_user"

type Handler struct {
	service          *Service
	cookieSecureMode string
	proxy            *proxy.Resolver
	ttl              time.Duration
}

func NewHandler(service *Service, cookieSecureMode string, resolver *proxy.Resolver, ttl time.Duration) *Handler {
	return &Handler{service: service, cookieSecureMode: cookieSecureMode, proxy: resolver, ttl: ttl}
}

type credentials struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *Handler) Login(c *gin.Context) {
	var in credentials
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password are required"})
		return
	}
	u, token, err := h.service.Login(in.Username, in.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}
	h.setCookie(c, token, int(h.ttl.Seconds()))
	c.JSON(http.StatusOK, u)
}
func (h *Handler) Logout(c *gin.Context) {
	token, _ := c.Cookie(CookieName)
	if err := h.service.Logout(token); err != nil {
		c.JSON(500, gin.H{"error": "could not end session"})
		return
	}
	h.setCookie(c, "", -1)
	c.Status(http.StatusNoContent)
}
func (h *Handler) Me(c *gin.Context) { c.JSON(http.StatusOK, CurrentUser(c)) }
func (h *Handler) ChangePassword(c *gin.Context) {
	var in struct {
		CurrentPassword string `json:"current_password" binding:"required"`
		NewPassword     string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(400, gin.H{"error": "current_password and new_password are required"})
		return
	}
	if err := h.service.ChangePassword(CurrentUser(c).ID, in.CurrentPassword, in.NewPassword); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	h.setCookie(c, "", -1)
	c.Status(http.StatusNoContent)
}
func (h *Handler) RequireUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(CookieName)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "authentication required"})
			return
		}
		u, err := h.service.UserForToken(token)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "authentication required"})
			return
		}
		c.Set(userKey, u)
		c.Next()
	}
}
func (h *Handler) setCookie(c *gin.Context, value string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(CookieName, value, maxAge, "/", "", h.cookieSecure(c.Request), true)
}
func (h *Handler) cookieSecure(req *http.Request) bool {
	switch h.cookieSecureMode {
	case "always":
		return true
	case "never":
		return false
	default:
		return h.proxy != nil && h.proxy.HTTPS(req)
	}
}
func CurrentUser(c *gin.Context) database.User {
	v, _ := c.Get(userKey)
	u, _ := v.(database.User)
	return u
}
