package handlers

import (
	"Site/db"
	"Site/models"
	"context"
	"html/template"
	"net/http"
	"strconv"
)

// AdminViewData — контекст для admin.html
type AdminViewData struct {
	ActiveTab string
	Posts     []models.Post
	Edit      *models.Post
	Projects  []models.Project
}

// AdminDashboard — единая точка входа в админку.
// tab=projects → проекты, иначе → посты
func AdminDashboard(w http.ResponseWriter, r *http.Request) {
	tab := r.URL.Query().Get("tab")
	data := AdminViewData{ActiveTab: tab}

	if tab == "projects" {
		rows, err := db.Pool.Query(context.Background(), `
    SELECT id, repo_name, title, description, image_url,
           github_url, custom_url, enabled
      FROM projects
  ORDER BY updated_at DESC`)

		if err != nil {
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
	} else {
		// таб «Посты»
		data.ActiveTab = "posts"
		rows, err := db.Pool.Query(context.Background(),
			"SELECT id,label,text FROM post ORDER BY id DESC")
		if err != nil {
			http.Error(w, "DB error", http.StatusInternalServerError)
			return
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
				db.Pool.QueryRow(context.Background(),
					"SELECT id,label,text FROM post WHERE id=$1", id).
					Scan(&e.ID, &e.Label, &e.Text)
				data.Edit = &e
			}
		}
	}

	tmpl := template.Must(template.ParseFiles("templates/admin/admin.html"))
	tmpl.ExecuteTemplate(w, "admin", data)
}
