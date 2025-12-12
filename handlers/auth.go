package handlers

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"
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
	userInput, pass := r.Form.Get("email"), r.Form.Get("password")

	var id int
	var hash, role string
	// Сначала ищем по email, затем по username для обратной совместимости
	err := db.Pool.QueryRow(context.Background(),
		"SELECT id,password_hash,role FROM users WHERE email=$1", userInput).
		Scan(&id, &hash, &role)
	if err != nil {
		err = db.Pool.QueryRow(context.Background(),
			"SELECT id,password_hash,role FROM users WHERE username=$1", userInput).
			Scan(&id, &hash, &role)
	}
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
	http.Redirect(w, r, "/cabinet", 303)
}

// RequireAuth — middleware, пропускает любого залогиненного пользователя (любая роль)
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("session_token")
		if err != nil || c.Value == "" {
			http.Redirect(w, r, "/login", 303)
			return
		}
		if _, err := parseToken(c.Value); err != nil {
			http.Redirect(w, r, "/login", 303)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// CurrentUserID — утилита для извлечения user_id из куки-JWT
func CurrentUserID(r *http.Request) (int, bool) {
	c, err := r.Cookie("session_token")
	if err != nil || c.Value == "" {
		return 0, false
	}
	claims, err := parseToken(c.Value)
	if err != nil {
		return 0, false
	}
	return claims.UserID, true
}

// RegisterHandler — регистрация нового пользователя
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		tmpl := template.Must(template.ParseFiles("templates/admin/register.html"))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tmpl.Execute(w, nil)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", 400)
		return
	}
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	if email == "" || password == "" {
		http.Error(w, "Заполните все поля", 400)
		return
	}
	if !strings.Contains(email, "@") {
		http.Error(w, "Некорректная почта", 400)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Server error", 500)
		return
	}
	// Пытаемся создать пользователя с ролью user
	var newID int
	err = db.Pool.QueryRow(context.Background(),
		"INSERT INTO users(username,email,password_hash,role) VALUES($1,$2,$3,'user') RETURNING id",
		email, email, string(hash)).Scan(&newID)
	if err != nil {
		http.Error(w, "Имя занято или ошибка БД", 400)
		return
	}
	// Автовход после регистрации (выдаём токен как при логине)
	exp := time.Now().Add(time.Hour)
	claims := &Claims{
		UserID: newID,
		Role:   "user",
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
	http.Redirect(w, r, "/cabinet", 303)
}

// LogoutHandler — завершает сессию (очищает JWT-куку)
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	// инвалидация куки
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(-time.Hour),
		MaxAge:   -1,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
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
