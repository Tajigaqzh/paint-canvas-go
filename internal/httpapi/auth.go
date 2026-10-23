package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"paint-canvas-go/internal/model"
)

const userIDContextKey = "authenticatedUserID"

type authInput struct {
	Email       string `json:"email" binding:"required,max=254"`
	Password    string `json:"password" binding:"required,min=8,max=72"`
	DisplayName string `json:"displayName" binding:"max=80"`
}

type authClaims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

func register(db *gorm.DB, secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input authInput
		if err := c.ShouldBindJSON(&input); err != nil {
			fail(c, http.StatusBadRequest, "invalid_request", "email and a password of at least 8 characters are required")
			return
		}
		email := strings.ToLower(strings.TrimSpace(input.Email))
		if !strings.Contains(email, "@") || strings.HasPrefix(email, "@") || strings.HasSuffix(email, "@") {
			fail(c, http.StatusBadRequest, "invalid_email", "email address is invalid")
			return
		}
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		if err != nil {
			fail(c, http.StatusInternalServerError, "auth_error", "could not create account")
			return
		}
		displayName := strings.TrimSpace(input.DisplayName)
		if displayName == "" {
			displayName = strings.Split(email, "@")[0]
		}
		user := model.User{
			ID:           uuid.NewString(),
			Email:        email,
			DisplayName:  displayName,
			PasswordHash: string(passwordHash),
		}
		if err := db.WithContext(c.Request.Context()).Create(&user).Error; err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "unique") {
				fail(c, http.StatusConflict, "email_taken", "an account with this email already exists")
				return
			}
			fail(c, http.StatusInternalServerError, "database_error", "could not create account")
			return
		}
		writeAuthResponse(c, user, secret)
	}
}

func login(db *gorm.DB, secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input authInput
		if err := c.ShouldBindJSON(&input); err != nil {
			fail(c, http.StatusBadRequest, "invalid_request", "email and password are required")
			return
		}
		var user model.User
		email := strings.ToLower(strings.TrimSpace(input.Email))
		if err := db.WithContext(c.Request.Context()).First(&user, "email = ?", email).Error; err != nil ||
			bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)) != nil {
			fail(c, http.StatusUnauthorized, "invalid_credentials", "email or password is incorrect")
			return
		}
		writeAuthResponse(c, user, secret)
	}
}

func writeAuthResponse(c *gin.Context, user model.User, secret string) {
	now := time.Now()
	claims := authClaims{
		Email: user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(12 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "paint-canvas-go",
			Subject:   user.ID,
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		fail(c, http.StatusInternalServerError, "auth_error", "could not create session")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"expiresAt": claims.ExpiresAt.Time,
		"user": gin.H{
			"id":          user.ID,
			"email":       user.Email,
			"displayName": user.DisplayName,
		},
	})
}

func requireAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := strings.TrimSpace(c.GetHeader("Authorization"))
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			fail(c, http.StatusUnauthorized, "unauthorized", "sign in to continue")
			c.Abort()
			return
		}
		token, err := jwt.ParseWithClaims(parts[1], &authClaims{}, func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		}, jwt.WithIssuer("paint-canvas-go"))
		if err != nil || !token.Valid {
			fail(c, http.StatusUnauthorized, "unauthorized", "session is invalid or expired")
			c.Abort()
			return
		}
		claims, ok := token.Claims.(*authClaims)
		if !ok || claims.Subject == "" {
			fail(c, http.StatusUnauthorized, "unauthorized", "session is invalid or expired")
			c.Abort()
			return
		}
		c.Set(userIDContextKey, claims.Subject)
		c.Next()
	}
}

func currentUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var user model.User
		if err := db.WithContext(c.Request.Context()).First(&user, "id = ?", c.GetString(userIDContextKey)).Error; err != nil {
			fail(c, http.StatusUnauthorized, "unauthorized", "account no longer exists")
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"id":          user.ID,
			"email":       user.Email,
			"displayName": user.DisplayName,
		})
	}
}
