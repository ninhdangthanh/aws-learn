package controller

import (
	"net/http"
	"strconv"

	"github.com/go-template/config"
	"github.com/go-template/service"

	"github.com/gin-gonic/gin"
)

type AuthController struct {
	svc service.UserService
	cfg *config.Config
}

func NewAuthController(cfg *config.Config) *AuthController {
	return &AuthController{
		svc: service.NewUserService(),
		cfg: cfg,
	}
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (ctrl *AuthController) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := ctrl.svc.Login(c.Request.Context(), req.Email, req.Password, ctrl.cfg.JWTSecret, ctrl.cfg.JWTExpirationHours, ctrl.cfg.RefreshExpirationDays)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (ctrl *AuthController) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := ctrl.svc.RefreshToken(c.Request.Context(), req.RefreshToken, ctrl.cfg.JWTSecret, ctrl.cfg.JWTExpirationHours)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (ctrl *AuthController) Logout(c *gin.Context) {
	userID, _ := c.Get("userID")
	jti, _ := c.Get("jti")

	// Note: Fully stateful logout should ideally revoke Refresh tokens too.
	// For simplicity, we just revoke the specific access token session.
	if err := ctrl.svc.Logout(c.Request.Context(), userID.(uint), jti.(string), ""); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Successfully logged out"})
}

func (ctrl *AuthController) EvictUser(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := ctrl.svc.EvictUser(c.Request.Context(), uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to evict user"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "User sessions evicted successfully"})
}
