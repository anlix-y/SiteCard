package handlers

import (
	"Site/db"
	"Site/logger"
	"Site/models"
	"context"
	"html/template"
	"net/http"
	"strconv"
	"strings"
)

// AdminViewData — контекст для admin.html
type AdminViewData struct {
	ActiveTab    string
	Posts        []models.Post
	Edit         *models.Post
	Projects     []models.Project
	Settings     *models.Settings
	IsAdmin      bool
	CurrentLogin string
	Error        string
}

// AdminDashboard — единая точка входа в админку.
// tab=projects → проекты, иначе → посты
func AdminDashboard(w http.ResponseWriter, r *http.Request) {
	tab := r.URL.Query().Get("tab")
	// Мы уже находимся за RequireAdmin — значит это админ
	isAdmin := true
	uid := 0
	if id, ok := CurrentUserID(r); ok {
		uid = id
	}
	// Получим логин (email или username)
	login := ""
	if uid > 0 {
		_ = db.Pool.QueryRow(context.Background(),
			"SELECT COALESCE(NULLIF(email,''), username, '') FROM users WHERE id=$1", uid,
		).Scan(&login)
	}
	data := AdminViewData{ActiveTab: tab, IsAdmin: isAdmin, CurrentLogin: login}

	if tab == "projects" {
		rows, err := db.Pool.Query(context.Background(), `
    SELECT id, repo_name, title, description, image_url,
           github_url, custom_url, enabled
      FROM projects
  ORDER BY updated_at DESC`)

		if err != nil {
			logger.Errorf("AdminDashboard projects: query error: %v", err)
			http.Error(w, "DB error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var p models.Project
			rows.Scan(&p.ID, &p.RepoName, &p.Title, &p.Description,
				&p.ImageURL, &p.GitHubURL, &p.CustomURL, &p.Enabled)
			data.Projects = append(data.Projects, p)
		}
	} else if tab == "setting" {
		// Вкладка Настройки — загружаем настройки текущего пользователя (админ тоже пользователь)
		if uid, ok := CurrentUserID(r); ok {
			var s models.Settings
			_ = db.Pool.QueryRow(context.Background(),
				"SELECT user_id, COALESCE(home_bg_url,''), COALESCE(link_github,''), COALESCE(link_tg,''), COALESCE(link_custom,''), COALESCE(slug,'') FROM settings WHERE user_id=$1",
				uid,
			).Scan(&s.UserID, &s.HomeBgURL, &s.LinkGitHub, &s.LinkTG, &s.LinkCustom, &s.Slug)
			if s.UserID == 0 {
				s.UserID = uid
			}
			data.Settings = &s
		}
	} else {
		// таб «Посты»
		data.ActiveTab = "posts"
		// Показываем посты текущего пользователя; для совместимости также подхватим старые глобальные (user_id IS NULL)
		rows, err := db.Pool.Query(context.Background(),
			"SELECT id,label,text FROM post WHERE user_id=$1 OR user_id IS NULL ORDER BY id DESC", uid)
		if err != nil {
			// Фоллбек для старых схем БД без столбца user_id
			if strings.Contains(err.Error(), "column \"user_id\" does not exist") ||
				strings.Contains(strings.ToLower(err.Error()), "undefined column") {
				rows, err = db.Pool.Query(context.Background(),
					"SELECT id,label,text FROM post ORDER BY id DESC")
			}
			if err != nil {
				logger.Errorf("AdminDashboard posts: query error: %v", err)
				http.Error(w, "DB error", http.StatusInternalServerError)
				return
			}
		}
		defer rows.Close()

		for rows.Next() {
			var p models.Post
			rows.Scan(&p.ID, &p.Label, &p.Text)
			data.Posts = append(data.Posts, p)
		}
		if editID := r.URL.Query().Get("edit_id"); editID != "" {
			if id, err := strconv.Atoi(editID); err == nil {
				var e models.Post
				// Разрешаем редактировать свои и (для обратной совместимости) старые глобальные посты
				db.Pool.QueryRow(context.Background(),
					"SELECT id,label,text FROM post WHERE id=$1 AND (user_id=$2 OR user_id IS NULL)", id, uid).
					Scan(&e.ID, &e.Label, &e.Text)
				data.Edit = &e
			}
		}
	}

	tmpl := template.Must(template.ParseFiles("templates/admin/admin.html"))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.ExecuteTemplate(w, "admin", data)
}
