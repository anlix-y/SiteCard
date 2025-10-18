package handlers

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"time"

	"Site/db"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var jwtKey = []byte(func() string {
	if s := os.Getenv("JWT_SECRET"); s != "" {
		return s
	}
	return "default_jwt_secret"
}())

type Claims struct {
	UserID int    `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func parseToken(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	tok, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected alg: %v", t.Header["alg"])
		}
		return jwtKey, nil
	})
	if err != nil || !tok.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		tmpl := template.Must(template.ParseFiles("templates/admin/loginAdmin.html"))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tmpl.Execute(w, nil)
		return
	}
	// POST
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", 400)
		return
	}
	user, pass := r.Form.Get("login"), r.Form.Get("password")

	var id int
	var hash, role string
	err := db.Pool.QueryRow(context.Background(),
		"SELECT id,password_hash,role FROM users WHERE username=$1", user).
		Scan(&id, &hash, &role)
	if err != nil {
		http.Error(w, "Invalid credentials", 401)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(pass)) != nil {
		http.Error(w, "Invalid credentials", 401)
		return
	}

	exp := time.Now().Add(time.Hour)
	claims := &Claims{
		UserID: id,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(exp),
			Issuer:    "site-admin",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokStr, err := token.SignedString(jwtKey)
	if err != nil {
		http.Error(w, "Server error", 500)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    tokStr,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Expires:  exp,
	})
	http.Redirect(w, r, "/admin", 303)
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("session_token")
		if err != nil || c.Value == "" {
			http.Redirect(w, r, "/login", 303)
			return
		}
		claims, err := parseToken(c.Value)
		if err != nil || claims.Role != "admin" {
			http.Redirect(w, r, "/login", 303)
			return
		}
		next.ServeHTTP(w, r)
	})
}
