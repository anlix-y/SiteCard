package handlers

import (
	"context"
	"html/template"
	"net/http"
	"strings"
	"unicode/utf8"

	"Site/db"
	"Site/logger"
	"Site/models"
	"github.com/gorilla/mux"
	"github.com/russross/blackfriday/v2"
)

type IndexPageData struct {
	Posts    []models.Post
	Settings *models.Settings
}

func Index(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Pool.Query(context.Background(),
		"SELECT id,label,text FROM post ORDER BY id DESC")
	if err != nil {
		logger.Errorf("Index: load posts: %v", err)
		http.Error(w, "DB error", 500)
		return
	}
	defer rows.Close()

	var posts []models.Post
	for rows.Next() {
		var p models.Post
		rows.Scan(&p.ID, &p.Label, &p.Text)
		posts = append(posts, p)
	}

	// Получаем настройки, если пользователь залогинен
	var settings *models.Settings
	if uid, ok := CurrentUserID(r); ok {
		var s models.Settings
		_ = db.Pool.QueryRow(context.Background(),
			"SELECT user_id, COALESCE(home_bg_url,''), COALESCE(link_github,''), COALESCE(link_tg,''), COALESCE(link_custom,'') FROM settings WHERE user_id=$1",
			uid,
		).Scan(&s.UserID, &s.HomeBgURL, &s.LinkGitHub, &s.LinkTG, &s.LinkCustom)
		if s.UserID != 0 {
			settings = &s
		}
	}

	tmpl := template.New("").Funcs(template.FuncMap{
		"markdown": func(s string) template.HTML {
			return template.HTML(blackfriday.Run([]byte(s)))
		},
		"truncate": func(s string, n int) string {
			if utf8.RuneCountInString(s) <= n {
				return s
			}
			rs := []rune(s)
			return string(rs[:n]) + "..."
		},
	})
	tmpl = template.Must(tmpl.ParseFiles(
		"templates/header.html",
		"templates/index.html",
		"templates/footer.html",
	))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.ExecuteTemplate(w, "index", IndexPageData{Posts: posts, Settings: settings})
}

// RootHandler — по требованию: "/" открывает логин/регистрацию, а для залогиненных — кабинет
func RootHandler(w http.ResponseWriter, r *http.Request) {
	// Если авторизован — направляем по роли:
	if c, err := r.Cookie("session_token"); err == nil && c.Value != "" {
		if cl, err := parseToken(c.Value); err == nil {
			if cl.Role == "admin" {
				http.Redirect(w, r, "/admin?tab=posts", http.StatusSeeOther)
				return
			}
			http.Redirect(w, r, "/cabinet", http.StatusSeeOther)
			return
		}
	}
	// гость — показываем страницу входа
	LoginHandler(w, r.WithContext(r.Context()))
}

// PublicProfile — публичная страница пользователя по слагу: "/{slug}"
func PublicProfile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	slug := vars["slug"]

	// Защитим системные маршруты
	switch slug {
	case "admin", "login", "register", "cabinet", "projects", "uploads", "static":
		http.NotFound(w, r)
		return
	}

	var s models.Settings
	err := db.Pool.QueryRow(context.Background(),
		"SELECT user_id, COALESCE(home_bg_url,''), COALESCE(link_github,''), COALESCE(link_tg,''), COALESCE(link_custom,''), COALESCE(slug,'') FROM settings WHERE slug=$1",
		slug,
	).Scan(&s.UserID, &s.HomeBgURL, &s.LinkGitHub, &s.LinkTG, &s.LinkCustom, &s.Slug)
	if err != nil {
		// Совместимость со старыми БД: если нет колонки или таблицы — отдаём 404
		e := strings.ToLower(err.Error())
		if strings.Contains(e, "undefined column") ||
			strings.Contains(e, "relation \"settings\" does not exist") ||
			strings.Contains(e, "does not exist") {
			http.NotFound(w, r)
			return
		}
	}
	if err != nil || s.UserID == 0 {
		http.NotFound(w, r)
		return
	}

	// Готовим список постов ТОЛЬКО данного пользователя
	rows, err := db.Pool.Query(context.Background(),
		"SELECT id,label,text FROM post WHERE user_id=$1 ORDER BY id DESC", s.UserID)
	if err != nil {
		// Обратная совместимость: если в базе нет столбца user_id — берём все посты
		if strings.Contains(strings.ToLower(err.Error()), "undefined column") {
			rows, err = db.Pool.Query(context.Background(),
				"SELECT id,label,text FROM post ORDER BY id DESC")
		}
		// Если таблицы post нет вовсе — отдаём пустой список, а не 500
		if err != nil && (strings.Contains(strings.ToLower(err.Error()), "relation \"post\" does not exist") ||
			strings.Contains(strings.ToLower(err.Error()), "does not exist")) {
			rows = nil
			err = nil
		}
		if err != nil {
			logger.Errorf("PublicProfile: posts query failed: %v", err)
			http.Error(w, "DB error", http.StatusInternalServerError)
			return
		}
	}
	if rows != nil {
		defer rows.Close()
	}

	var posts []models.Post
	if rows != nil {
		for rows.Next() {
			var p models.Post
			rows.Scan(&p.ID, &p.Label, &p.Text)
			posts = append(posts, p)
		}
	}

	tmpl := template.New("").Funcs(template.FuncMap{
		"markdown": func(s string) template.HTML {
			return template.HTML(blackfriday.Run([]byte(s)))
		},
		"truncate": func(s string, n int) string {
			if utf8.RuneCountInString(s) <= n {
				return s
			}
			rs := []rune(s)
			return string(rs[:n]) + "..."
		},
	})
	tmpl = template.Must(tmpl.ParseFiles(
		"templates/header.html",
		"templates/index.html",
		"templates/footer.html",
	))
	// Передаём посты и настройки пользователя
	data := IndexPageData{Posts: posts, Settings: &s}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.ExecuteTemplate(w, "index", data)
}
