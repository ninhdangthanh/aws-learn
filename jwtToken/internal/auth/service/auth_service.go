package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ninhdang/jwt-auth/internal/auth/model"
	"github.com/ninhdang/jwt-auth/internal/auth/repository"
	"github.com/ninhdang/jwt-auth/internal/user"
	"github.com/ninhdang/jwt-auth/pkg/jwt"
	"github.com/ninhdang/jwt-auth/pkg/password"
)

// Lỗi domain — handler map các lỗi này sang HTTP status.
var (
	ErrEmailTaken         = errors.New("service: email already registered")
	ErrInvalidCredentials = errors.New("service: invalid email or password")
	ErrInvalidRefresh     = errors.New("service: refresh token is invalid or expired")
	ErrUserNotFound       = errors.New("service: user not found")
	ErrSamePassword       = errors.New("service: new password must differ from the current one")
)

// RequestMeta là thông tin ngữ cảnh của request, dùng cho audit phiên đăng nhập.
type RequestMeta struct {
	UserAgent string
	IP        string
}

// AuthService chứa toàn bộ business logic của Phase 1:
// Register / Login / Refresh / Logout / Me.
type AuthService struct {
	users   *repository.UserRepository
	refresh *repository.RefreshRepository
	tokens  *repository.TokenStore
	jwt     *jwt.Manager
}

func NewAuthService(
	users *repository.UserRepository,
	refresh *repository.RefreshRepository,
	tokens *repository.TokenStore,
	jwtManager *jwt.Manager,
) *AuthService {
	return &AuthService{users: users, refresh: refresh, tokens: tokens, jwt: jwtManager}
}

// Register tạo tài khoản mới và đăng nhập luôn để FE không phải gọi 2 lần.
func (s *AuthService) Register(ctx context.Context, req model.RegisterRequest, meta RequestMeta) (*model.AuthResponse, error) {
	email := normalizeEmail(req.Email)

	exists, err := s.users.ExistsByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrEmailTaken
	}

	hash, err := password.Hash(req.Password)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	u := &user.User{
		ID:           uuid.NewString(),
		Email:        email,
		PasswordHash: hash,
		FullName:     strings.TrimSpace(req.FullName),
		TokenVersion: 1,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, err
	}

	return s.issueSession(ctx, u, "", meta)
}

// Login: verify password -> phát access + refresh token -> lưu refresh vào Redis.
func (s *AuthService) Login(ctx context.Context, req model.LoginRequest, meta RequestMeta) (*model.AuthResponse, error) {
	u, err := s.users.FindByEmail(ctx, normalizeEmail(req.Email))
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			// Trả cùng một lỗi cho "email sai" và "mật khẩu sai" để không lộ
			// email nào đã tồn tại (user enumeration).
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if err := password.Compare(u.PasswordHash, req.Password); err != nil {
		return nil, ErrInvalidCredentials
	}

	return s.issueSession(ctx, u, req.DeviceID, meta)
}

// Refresh (Bài 2 + Bài 3): verify refresh token -> kiểm tra Redis -> xoay vòng.
//
// Rotation: refresh token cũ bị xoá khỏi Redis và thay bằng một jti hoàn toàn
// mới. Mỗi refresh token vì thế chỉ dùng được đúng một lần — kẻ đánh cắp có
// dùng lại token cũ cũng chỉ nhận 401.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*model.TokenPair, error) {
	claims, err := s.jwt.Parse(refreshToken, jwt.TokenTypeRefresh)
	if err != nil {
		return nil, ErrInvalidRefresh
	}

	// Chữ ký hợp lệ chưa đủ: token phải còn tồn tại trong Redis.
	sess, err := s.tokens.GetRefresh(ctx, claims.ID)
	if err != nil {
		if errors.Is(err, repository.ErrRefreshNotFound) {
			return nil, ErrInvalidRefresh
		}
		return nil, err
	}
	if sess.UserID != claims.Subject {
		return nil, ErrInvalidRefresh
	}

	u, err := s.users.FindByID(ctx, claims.Subject)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrInvalidRefresh
		}
		return nil, err
	}
	// Token phát ra trước khi user logout-all thì không được phép đổi lấy token mới.
	if claims.TokenVersion != u.TokenVersion {
		return nil, ErrInvalidRefresh
	}

	// Xoá token cũ TRƯỚC khi phát token mới: nếu bước sau lỗi, user phải đăng
	// nhập lại — chấp nhận được. Ngược lại (phát mới rồi mới xoá) sẽ có cửa sổ
	// tồn tại hai refresh token cùng hiệu lực.
	if err := s.tokens.DeleteRefresh(ctx, claims.ID, claims.Subject); err != nil {
		return nil, err
	}
	if err := s.refresh.Revoke(ctx, claims.ID); err != nil {
		return nil, err
	}

	res, err := s.issueSession(ctx, u, sess.DeviceID, RequestMeta{
		UserAgent: sess.UserAgent,
		IP:        sess.IP,
	})
	if err != nil {
		return nil, err
	}
	return &res.Tokens, nil
}

// Logout (Bài 8): xoá refresh token + đưa access token hiện tại vào blacklist.
// Không blacklist thì access token vẫn sống thêm tới 15 phút sau khi bấm
// "đăng xuất" — đúng nghĩa là chưa đăng xuất.
func (s *AuthService) Logout(ctx context.Context, access *jwt.Claims, refreshToken string) error {
	if err := s.blacklistAccess(ctx, access); err != nil {
		return err
	}

	claims, err := s.jwt.Parse(refreshToken, jwt.TokenTypeRefresh)
	if err != nil {
		// Refresh token rác/hết hạn: access đã bị chặn rồi, coi như xong.
		return nil
	}
	// Chỉ cho xoá refresh token của chính mình.
	if claims.Subject != access.Subject {
		return nil
	}

	if err := s.tokens.DeleteRefresh(ctx, claims.ID, claims.Subject); err != nil {
		return err
	}
	return s.refresh.Revoke(ctx, claims.ID)
}

// LogoutAll (Bài 9): tăng token_version + xoá sạch refresh token của user.
//
// token_version++ vô hiệu hoá mọi access token đang lưu hành trong một thao tác
// duy nhất — không cần biết chúng là những jti nào, cũng không cần blacklist
// từng cái. Đây là lý do token_version tồn tại.
func (s *AuthService) LogoutAll(ctx context.Context, userID string) error {
	newVersion, err := s.users.IncrementTokenVersion(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrUserNotFound
		}
		return err
	}

	// Ghi đè cache ngay để lệnh thu hồi có hiệu lực tức thì, không phải chờ TTL.
	if err := s.tokens.SetTokenVersion(ctx, userID, newVersion, tokenVersionCacheTTL); err != nil {
		return err
	}

	return s.revokeAllRefresh(ctx, userID)
}

// ChangePassword đổi mật khẩu rồi đá mọi phiên khác ra ngoài, và cấp cho thiết
// bị hiện tại một cặp token mới để user không bị tự đăng xuất chính mình.
func (s *AuthService) ChangePassword(
	ctx context.Context,
	userID string,
	req model.ChangePasswordRequest,
	meta RequestMeta,
) (*model.AuthResponse, error) {
	u, err := s.users.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	if err := password.Compare(u.PasswordHash, req.CurrentPassword); err != nil {
		return nil, ErrInvalidCredentials
	}
	if req.CurrentPassword == req.NewPassword {
		return nil, ErrSamePassword
	}

	hash, err := password.Hash(req.NewPassword)
	if err != nil {
		return nil, err
	}
	if err := s.users.UpdatePassword(ctx, u.ID, hash); err != nil {
		return nil, err
	}

	// Đổi mật khẩu thường là phản ứng khi nghi bị lộ tài khoản → thu hồi tất cả.
	newVersion, err := s.users.IncrementTokenVersion(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	if err := s.tokens.SetTokenVersion(ctx, u.ID, newVersion, tokenVersionCacheTTL); err != nil {
		return nil, err
	}
	if err := s.revokeAllRefresh(ctx, u.ID); err != nil {
		return nil, err
	}

	u.PasswordHash = hash
	u.TokenVersion = newVersion
	return s.issueSession(ctx, u, req.DeviceID, meta)
}

// revokeAllRefresh xoá refresh token của user ở cả Redis lẫn PostgreSQL.
func (s *AuthService) revokeAllRefresh(ctx context.Context, userID string) error {
	if _, err := s.tokens.DeleteAllRefreshByUser(ctx, userID); err != nil {
		return err
	}
	return s.refresh.RevokeAllByUser(ctx, userID)
}

// blacklistAccess chặn access token hiện tại trong đúng thời gian sống còn lại.
func (s *AuthService) blacklistAccess(ctx context.Context, access *jwt.Claims) error {
	if access == nil || access.ExpiresAt == nil {
		return nil
	}
	return s.tokens.BlacklistAccess(ctx, access.ID, time.Until(access.ExpiresAt.Time))
}

// Me trả về profile của user đang đăng nhập.
func (s *AuthService) Me(ctx context.Context, userID string) (*user.PublicUser, error) {
	u, err := s.users.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	pub := u.Public()
	return &pub, nil
}

// Sessions liệt kê các thiết bị đang đăng nhập của user.
func (s *AuthService) Sessions(ctx context.Context, userID string) ([]model.SessionInfo, error) {
	rows, err := s.refresh.ListActiveByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	sessions := make([]model.SessionInfo, 0, len(rows))
	for _, row := range rows {
		// Chỉ hiện phiên nào Redis còn giữ — DB có thể còn hàng cũ chưa dọn.
		if _, err := s.tokens.GetRefresh(ctx, row.JTI); err != nil {
			continue
		}
		sessions = append(sessions, model.SessionInfo{
			JTI:       row.JTI,
			DeviceID:  row.DeviceID,
			UserAgent: row.UserAgent,
			IP:        row.IP,
			CreatedAt: row.CreatedAt,
			ExpiresAt: row.ExpiresAt,
		})
	}
	return sessions, nil
}

// issueSession sinh cặp token và ghi refresh token xuống Redis + PostgreSQL.
func (s *AuthService) issueSession(ctx context.Context, u *user.User, deviceID string, meta RequestMeta) (*model.AuthResponse, error) {
	if deviceID == "" {
		deviceID = uuid.NewString()
	}

	access, err := s.jwt.GenerateAccessToken(u.ID, u.Email, u.TokenVersion)
	if err != nil {
		return nil, err
	}
	refreshToken, err := s.jwt.GenerateRefreshToken(u.ID, deviceID, u.TokenVersion)
	if err != nil {
		return nil, err
	}

	// Login lại trên cùng thiết bị thì phiên cũ của thiết bị đó không còn ý nghĩa.
	if err := s.refresh.RevokeByUserAndDevice(ctx, u.ID, deviceID); err != nil {
		return nil, err
	}

	ttl := time.Until(refreshToken.ExpiresAt)
	err = s.tokens.SaveRefresh(ctx, refreshToken.JTI, repository.RefreshSession{
		UserID:    u.ID,
		DeviceID:  deviceID,
		UserAgent: meta.UserAgent,
		IP:        meta.IP,
		IssuedAt:  time.Now(),
	}, ttl)
	if err != nil {
		return nil, err
	}

	record := &model.RefreshToken{
		ID:        uuid.NewString(),
		UserID:    u.ID,
		JTI:       refreshToken.JTI,
		DeviceID:  deviceID,
		UserAgent: meta.UserAgent,
		IP:        meta.IP,
		ExpiresAt: refreshToken.ExpiresAt,
		CreatedAt: time.Now(),
	}
	if err := s.refresh.Create(ctx, record); err != nil {
		// Redis đã ghi rồi, DB fail thì rollback thủ công để hai bên không lệch nhau.
		_ = s.tokens.DeleteRefresh(ctx, refreshToken.JTI, u.ID)
		return nil, fmt.Errorf("service: persist refresh token: %w", err)
	}

	return &model.AuthResponse{
		User: u.Public(),
		Tokens: model.TokenPair{
			AccessToken:  access.Value,
			RefreshToken: refreshToken.Value,
			TokenType:    "Bearer",
			ExpiresIn:    int(s.jwt.AccessTTL().Seconds()),
			DeviceID:     deviceID,
		},
	}, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
