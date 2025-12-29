package tokens

import (
	"crypto/rand"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/bcrypt"
	"math/big"
	"os"
	"time"
)

type TokenService interface {
	GenerateSecureOTP(length int) (string, error)
	ComparePasswords(storedPassword, candidatePassword string) bool
	GenerateToken(userID int, email string, verified bool, role string) (string, string)
	DecodeToken(tokenString string) (*Claims, error)
}

type Tokens struct{}

func NewTokenService() *Tokens {
	return &Tokens{}
}

func (t *Tokens) GenerateSecureOTP(length int) (string, error) {
	otp := ""
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10)) // Random digit (0-9)
		if err != nil {
			return "", err
		}
		otp += n.String()
	}
	return otp, nil
}

func (t *Tokens) ComparePasswords(storedPassword, candidatePassword string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(candidatePassword))

	if err != nil {
		return false
	}
	return true
}

func (t *Tokens) GenerateToken(userID int, email string, verified bool, role string) (string, string) {
	key := os.Getenv("SECRET_KEY")
	if key == "" {
		panic(errors.New("no secret key found"))
	}
	secretKey := []byte(key)

	tokenExpiry := time.Now().Add(24 * time.Hour).Unix()
	refreshTokenExpiry := time.Now().Add(7 * 24 * time.Hour).Unix()

	claims := &Claims{
		Email:    email,
		UserID:   userID,
		Role:     role,
		Verified: verified,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: tokenExpiry,
		},
	}

	refreshClaims := &Claims{
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: refreshTokenExpiry,
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedAccessToken, err := accessToken.SignedString(secretKey)
	if err != nil {
		panic(err)
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	signedRefreshToken, err := refreshToken.SignedString(secretKey)
	if err != nil {
		panic(err)
	}

	return signedAccessToken, signedRefreshToken
}

type Claims struct {
	UserID   int    `json:"user_id"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Verified bool   `json:"verified"`
	jwt.StandardClaims
}

func (t *Tokens) DecodeToken(tokenString string) (*Claims, error) {
	key := os.Getenv("SECRET_KEY")
	if key == "" {
		return nil, errors.New("no secret key found")
	}
	secretKey := []byte(key)

	// Parse token
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secretKey, nil
	})

	if err != nil {
		// Detailed error handling
		var ve *jwt.ValidationError
		if errors.As(err, &ve) {
			if ve.Errors&jwt.ValidationErrorExpired != 0 {
				return nil, errors.New("token has expired")
			}
			if ve.Errors&jwt.ValidationErrorSignatureInvalid != 0 {
				return nil, errors.New("invalid token signature")
			}
			if ve.Errors&jwt.ValidationErrorMalformed != 0 {
				return nil, errors.New("malformed token")
			}
		}
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// Extract claims
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, errors.New("invalid claims type")
	}

	if !token.Valid {
		return nil, errors.New("token is not valid")
	}

	return claims, nil
}
