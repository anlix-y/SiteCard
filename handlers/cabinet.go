package handlers

import (
	"Site/db"
	"Site/logger"
	"Site/models"
	"context"
	"html/template"
	"net/http"
	"regexp"
	"strings"
)

type CabinetViewData struct {
	Settings models.Settings
	Error    string
}

func CabinetPage(w http.ResponseWriter, r *http.Request) {
	uid, ok := CurrentUserID(r)
	if !ok {
		http.Redirect(w, r, "/login", 303)
		return
	}
	// По запросу: показываем вкладки «Посты» и «Проекты» всем ролям
	isAdmin := true
	// Логин текущего пользователя (email или username)
	login := ""
	_ = db.Pool.QueryRow(context.Background(),
		"SELECT COALESCE(NULLIF(email,''), username, '') FROM users WHERE id=$1", uid,
	).Scan(&login)
	var s models.Settings
	_ = db.Pool.QueryRow(context.Background(),
		"SELECT user_id, COALESCE(home_bg_url,''), COALESCE(link_github,''), COALESCE(link_tg,''), COALESCE(link_custom,''), COALESCE(slug,'') FROM settings WHERE user_id=$1",
		uid,
	).Scan(&s.UserID, &s.HomeBgURL, &s.LinkGitHub, &s.LinkTG, &s.LinkCustom, &s.Slug)
	// если настроек нет — установим user_id
	if s.UserID == 0 {
		s.UserID = uid
	}
	// Рендерим объединённый интерфейс (кабинет/админка) — вкладка «Настройки»
	data := AdminViewData{
		ActiveTab:    "setting",
		Settings:     &s,
		IsAdmin:      isAdmin,
		CurrentLogin: login,
	}
	tmpl := template.Must(template.ParseFiles("templates/admin/admin.html"))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.ExecuteTemplate(w, "admin", data)
}

func CabinetSave(w http.ResponseWriter, r *http.Request) {
	uid, ok := CurrentUserID(r)
	if !ok {
		http.Redirect(w, r, "/login", 303)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", 400)
		return
	}
	bg := r.FormValue("home_bg_url")
	gh := r.FormValue("link_github")
	tg := r.FormValue("link_tg")
	cu := r.FormValue("link_custom")
	slug := strings.TrimSpace(strings.ToLower(r.FormValue("slug")))

	// Валидация слага (может быть пустым)
	if slug != "" {
		// зарезервированные пути
		reserved := map[string]bool{
			"admin": true, "login": true, "register": true, "cabinet": true,
			"projects": true, "uploads": true, "static": true,
		}
		if reserved[slug] {
			renderCabinetError(w, uid, "Этот адрес зарезервирован, выберите другой")
			return
		}
		re := regexp.MustCompile(`^[a-z0-9-]{3,32}$`)
		if !re.MatchString(slug) {
			renderCabinetError(w, uid, "Адрес может содержать латинские буквы, цифры и дефис. Длина 3-32.")
			return
		}
		// проверка уникальности
		var exists int
		_ = db.Pool.QueryRow(context.Background(),
			"SELECT 1 FROM settings WHERE slug=$1 AND user_id<>$2", slug, uid).Scan(&exists)
		if exists == 1 {
			renderCabinetError(w, uid, "Такой адрес уже используется")
			return
		}
	}

	_, err := db.Pool.Exec(context.Background(), `
INSERT INTO settings(user_id, home_bg_url, link_github, link_tg, link_custom, slug)
VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT (user_id) DO UPDATE SET
  home_bg_url=EXCLUDED.home_bg_url,
  link_github=EXCLUDED.link_github,
  link_tg=EXCLUDED.link_tg,
  link_custom=EXCLUDED.link_custom,
  slug=EXCLUDED.slug
`, uid, bg, gh, tg, cu, slug)
	if err != nil {
		logger.Errorf("CabinetSave: upsert settings error (uid=%d): %v", uid, err)
		http.Error(w, "DB error", 500)
		return
	}
	http.Redirect(w, r, "/cabinet", 303)
}

func renderCabinetError(w http.ResponseWriter, uid int, msg string) {
	// По запросу: показываем вкладки «Посты» и «Проекты» всем ролям
	isAdmin := true
	// Логин текущего пользователя (email или username)
	login := ""
	_ = db.Pool.QueryRow(context.Background(),
		"SELECT COALESCE(NULLIF(email,''), username, '') FROM users WHERE id=$1", uid,
	).Scan(&login)
	var s models.Settings
	_ = db.Pool.QueryRow(context.Background(),
		"SELECT user_id, COALESCE(home_bg_url,''), COALESCE(link_github,''), COALESCE(link_tg,''), COALESCE(link_custom,''), COALESCE(slug,'') FROM settings WHERE user_id=$1",
		uid,
	).Scan(&s.UserID, &s.HomeBgURL, &s.LinkGitHub, &s.LinkTG, &s.LinkCustom, &s.Slug)
	if s.UserID == 0 {
		s.UserID = uid
	}
	data := AdminViewData{
		ActiveTab:    "setting",
		Settings:     &s,
		IsAdmin:      isAdmin,
		CurrentLogin: login,
		Error:        msg,
	}
	tmpl := template.Must(template.ParseFiles("templates/admin/admin.html"))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.ExecuteTemplate(w, "admin", data)
}
