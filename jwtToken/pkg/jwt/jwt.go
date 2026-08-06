// Package jwt bọc golang-jwt để phần còn lại của app không phụ thuộc trực tiếp
// vào thuật toán ký. Phase 1 dùng HS256; Bài 10 sẽ đổi sang RS256 bằng cách thay
// signingMethod + key mà không đụng tới service/middleware.
package jwt

import (
	"errors"
	"fmt"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenType phân biệt access và refresh token để không thể dùng lẫn lộn:
// một refresh token không được phép đi qua middleware như access token.
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

var (
	ErrInvalidToken     = errors.New("jwt: invalid token")
	ErrTokenExpired     = errors.New("jwt: token expired")
	ErrUnexpectedTyp    = errors.New("jwt: unexpected token type")
	ErrUnexpectedMethod = errors.New("jwt: unexpected signing method")
)

// Claims là payload custom của cả access lẫn refresh token.
type Claims struct {
	jwtlib.RegisteredClaims

	Email string    `json:"email,omitempty"`
	Type  TokenType `json:"typ"`
	// TokenVersion phục vụ Logout All (Bài 4). Phase 1 chỉ nhét vào claim,
	// chưa đối chiếu với DB trong middleware.
	TokenVersion int `json:"ver"`
	// DeviceID phục vụ multi-device (Bài 7), chỉ có trên refresh token.
	DeviceID string `json:"device_id,omitempty"`
}

// Token là kết quả sau khi ký: chuỗi JWT + metadata mà caller cần lưu lại.
type Token struct {
	Value     string
	JTI       string
	ExpiresAt time.Time
}

// Manager ký và verify token.
type Manager struct {
	secret     []byte
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewManager(secret, issuer string, accessTTL, refreshTTL time.Duration) *Manager {
	return &Manager{
		secret:     []byte(secret),
		issuer:     issuer,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

func (m *Manager) AccessTTL() time.Duration  { return m.accessTTL }
func (m *Manager) RefreshTTL() time.Duration { return m.refreshTTL }

// GenerateAccessToken ký access token ngắn hạn. Không có state ở server:
// middleware chỉ cần verify chữ ký (Bài 1).
func (m *Manager) GenerateAccessToken(userID, email string, tokenVersion int) (*Token, error) {
	return m.sign(claimsInput{
		userID:       userID,
		email:        email,
		tokenVersion: tokenVersion,
		typ:          TokenTypeAccess,
		ttl:          m.accessTTL,
	})
}

// GenerateRefreshToken ký refresh token dài hạn. JTI của nó chính là key lưu
// trong Redis (refresh:{jti}) nên refresh token là stateful, thu hồi được.
func (m *Manager) GenerateRefreshToken(userID, deviceID string, tokenVersion int) (*Token, error) {
	return m.sign(claimsInput{
		userID:       userID,
		tokenVersion: tokenVersion,
		typ:          TokenTypeRefresh,
		deviceID:     deviceID,
		ttl:          m.refreshTTL,
	})
}

type claimsInput struct {
	userID       string
	email        string
	tokenVersion int
	typ          TokenType
	deviceID     string
	ttl          time.Duration
}

func (m *Manager) sign(in claimsInput) (*Token, error) {
	now := time.Now()
	expiresAt := now.Add(in.ttl)
	jti := uuid.NewString()

	claims := &Claims{
		RegisteredClaims: jwtlib.RegisteredClaims{
			ID:        jti,
			Subject:   in.userID,
			Issuer:    m.issuer,
			IssuedAt:  jwtlib.NewNumericDate(now),
			NotBefore: jwtlib.NewNumericDate(now),
			ExpiresAt: jwtlib.NewNumericDate(expiresAt),
		},
		Email:        in.email,
		Type:         in.typ,
		TokenVersion: in.tokenVersion,
		DeviceID:     in.deviceID,
	}

	signed, err := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return nil, fmt.Errorf("jwt: sign %s token: %w", in.typ, err)
	}

	return &Token{Value: signed, JTI: jti, ExpiresAt: expiresAt}, nil
}

// Parse verify chữ ký + thời hạn, đồng thời ép đúng loại token mong đợi.
func (m *Manager) Parse(tokenString string, expected TokenType) (*Claims, error) {
	claims := &Claims{}

	_, err := jwtlib.ParseWithClaims(tokenString, claims, func(t *jwtlib.Token) (any, error) {
		if _, ok := t.Method.(*jwtlib.SigningMethodHMAC); !ok {
			return nil, ErrUnexpectedMethod
		}
		return m.secret, nil
	}, jwtlib.WithIssuer(m.issuer), jwtlib.WithExpirationRequired())

	if err != nil {
		if errors.Is(err, jwtlib.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	if claims.Type != expected {
		return nil, ErrUnexpectedTyp
	}
	if claims.Subject == "" || claims.ID == "" {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
