package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ninhdang/jwt-auth/internal/auth/middleware"
	"github.com/ninhdang/jwt-auth/internal/auth/model"
	"github.com/ninhdang/jwt-auth/internal/auth/service"
	"github.com/ninhdang/jwt-auth/pkg/jwt"
)

// AuthHandler map HTTP <-> AuthService.
type AuthHandler struct {
	svc   *service.AuthService
	jwt   *jwt.Manager
	guard middleware.Guard
}

func NewAuthHandler(svc *service.AuthService, jwtManager *jwt.Manager, guard middleware.Guard) *AuthHandler {
	return &AuthHandler{svc: svc, jwt: jwtManager, guard: guard}
}

// RegisterRoutes gắn toàn bộ route vào router group.
func (h *AuthHandler) RegisterRoutes(r *gin.RouterGroup) {
	// Public: chưa có (hoặc không còn) access token hợp lệ.
	public := r.Group("/auth")
	{
		public.POST("/register", h.Register)
		public.POST("/login", h.Login)
		public.POST("/refresh", h.Refresh)
	}

	protected := r.Group("")
	protected.Use(middleware.RequireAuth(h.jwt, h.guard))
	{
		// Logout cần access token để biết jti nào phải đưa vào blacklist.
		protected.POST("/auth/logout", h.Logout)
		protected.POST("/auth/logout-all", h.LogoutAll)
		protected.POST("/auth/change-password", h.ChangePassword)
		protected.GET("/me", h.Me)
		protected.GET("/sessions", h.Sessions)
	}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req model.RegisterRequest
	if !bindJSON(c, &req) {
		return
	}

	res, err := h.svc.Register(c.Request.Context(), req, requestMeta(c))
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, res)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req model.LoginRequest
	if !bindJSON(c, &req) {
		return
	}

	res, err := h.svc.Login(c.Request.Context(), req, requestMeta(c))
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req model.RefreshRequest
	if !bindJSON(c, &req) {
		return
	}

	tokens, err := h.svc.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"tokens": tokens})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req model.LogoutRequest
	if !bindJSON(c, &req) {
		return
	}

	if err := h.svc.Logout(c.Request.Context(), middleware.Claims(c), req.RefreshToken); err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Đăng xuất thành công"})
}

func (h *AuthHandler) LogoutAll(c *gin.Context) {
	if err := h.svc.LogoutAll(c.Request.Context(), middleware.UserID(c)); err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Đã đăng xuất khỏi tất cả thiết bị"})
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req model.ChangePasswordRequest
	if !bindJSON(c, &req) {
		return
	}

	res, err := h.svc.ChangePassword(c.Request.Context(), middleware.UserID(c), req, requestMeta(c))
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *AuthHandler) Me(c *gin.Context) {
	u, err := h.svc.Me(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": u})
}

func (h *AuthHandler) Sessions(c *gin.Context) {
	sessions, err := h.svc.Sessions(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		respondServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

func bindJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_failed",
			"message": err.Error(),
		})
		return false
	}
	return true
}

func requestMeta(c *gin.Context) service.RequestMeta {
	return service.RequestMeta{
		UserAgent: c.Request.UserAgent(),
		IP:        c.ClientIP(),
	}
}

// respondServiceError map lỗi domain sang HTTP status; lỗi lạ thì log lại và
// trả 500 chung chung để không rò rỉ chi tiết nội bộ ra client.
func respondServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrEmailTaken):
		c.JSON(http.StatusConflict, gin.H{"error": "email_taken", "message": "Email đã được đăng ký"})
	case errors.Is(err, service.ErrInvalidCredentials):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_credentials", "message": "Email hoặc mật khẩu không đúng"})
	case errors.Is(err, service.ErrInvalidRefresh):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_refresh_token", "message": "Refresh token không hợp lệ hoặc đã hết hạn"})
	case errors.Is(err, service.ErrUserNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "user_not_found", "message": "Không tìm thấy người dùng"})
	case errors.Is(err, service.ErrSamePassword):
		c.JSON(http.StatusBadRequest, gin.H{"error": "same_password", "message": "Mật khẩu mới phải khác mật khẩu hiện tại"})
	default:
		slog.Error("unhandled service error", "path", c.FullPath(), "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error", "message": "Có lỗi xảy ra, vui lòng thử lại"})
	}
}
