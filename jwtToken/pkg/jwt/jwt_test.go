package jwt

import (
	"errors"
	"testing"
	"time"
)

const testSecret = "test-secret-that-is-long-enough-for-hs256"

func newTestManager(accessTTL time.Duration) *Manager {
	return NewManager(testSecret, "jwt-auth-test", accessTTL, time.Hour)
}

func TestParseAccessTokenRoundTrip(t *testing.T) {
	m := newTestManager(time.Minute)

	tok, err := m.GenerateAccessToken("user-1", "a@b.com", 3)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	claims, err := m.Parse(tok.Value, TokenTypeAccess)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if claims.Subject != "user-1" {
		t.Errorf("sub = %q, want user-1", claims.Subject)
	}
	if claims.ID != tok.JTI {
		t.Errorf("jti = %q, want %q", claims.ID, tok.JTI)
	}
	if claims.Email != "a@b.com" {
		t.Errorf("email = %q, want a@b.com", claims.Email)
	}
	if claims.TokenVersion != 3 {
		t.Errorf("ver = %d, want 3", claims.TokenVersion)
	}
}

// Refresh token không được dùng thay access token: đây là ranh giới bảo mật,
// nếu thủng thì một token 7 ngày sẽ mở được mọi endpoint được bảo vệ.
func TestParseRejectsWrongTokenType(t *testing.T) {
	m := newTestManager(time.Minute)

	refresh, err := m.GenerateRefreshToken("user-1", "macbook", 1)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if _, err := m.Parse(refresh.Value, TokenTypeAccess); !errors.Is(err, ErrUnexpectedTyp) {
		t.Fatalf("err = %v, want ErrUnexpectedTyp", err)
	}
}

func TestParseRejectsExpiredToken(t *testing.T) {
	m := newTestManager(-time.Second) // đã hết hạn ngay khi tạo

	tok, err := m.GenerateAccessToken("user-1", "a@b.com", 1)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if _, err := m.Parse(tok.Value, TokenTypeAccess); !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("err = %v, want ErrTokenExpired", err)
	}
}

func TestParseRejectsTokenSignedWithAnotherSecret(t *testing.T) {
	signer := newTestManager(time.Minute)
	verifier := NewManager("a-completely-different-secret-value-32b", "jwt-auth-test", time.Minute, time.Hour)

	tok, err := signer.GenerateAccessToken("user-1", "a@b.com", 1)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if _, err := verifier.Parse(tok.Value, TokenTypeAccess); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("err = %v, want ErrInvalidToken", err)
	}
}

// Tấn công alg=none: bỏ chữ ký và khai báo thuật toán "none".
func TestParseRejectsNoneAlgorithm(t *testing.T) {
	m := newTestManager(time.Minute)

	// {"alg":"none","typ":"JWT"} . {...claims...} . (chữ ký rỗng)
	forged := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." +
		"eyJpc3MiOiJqd3QtYXV0aC10ZXN0Iiwic3ViIjoiaGFja2VyIiwiZXhwIjo5OTk5OTk5OTk5LCJqdGkiOiJ4IiwidHlwIjoiYWNjZXNzIn0."

	if _, err := m.Parse(forged, TokenTypeAccess); err == nil {
		t.Fatal("parse chấp nhận token alg=none, phải từ chối")
	}
}

func TestGenerateProducesUniqueJTI(t *testing.T) {
	m := newTestManager(time.Minute)

	first, _ := m.GenerateAccessToken("user-1", "a@b.com", 1)
	second, _ := m.GenerateAccessToken("user-1", "a@b.com", 1)

	if first.JTI == second.JTI {
		t.Fatal("hai token có cùng jti, không thể thu hồi riêng lẻ")
	}
}
