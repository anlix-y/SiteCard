package main

import (
	"Site/db"
	"Site/models"
	"context"
	"fmt"
	"html/template"
	"net/http"
)

func AdminProjects(w http.ResponseWriter, r *http.Request) {
	rows, _ := db.Pool.Query(context.Background(),
		"SELECT id,repo_name,title,description,image_url,github_url,custom_url,enabled,updated_at FROM projects ORDER BY updated_at DESC")
	defer rows.Close()

	var list []models.Project
	for rows.Next() {
		var p models.Project
		rows.Scan(&p.ID, &p.RepoName, &p.Title, &p.Description,
			&p.ImageURL, &p.GitHubURL, &p.CustomURL, &p.Enabled, &p.UpdatedAt)
		list = append(list, p)
	}

	tmpl := template.Must(template.ParseFiles(
		"templates/admin/admin.html", // ваш основной шаблон с tab-pane "projects"
	))
	tmpl.Execute(w, struct{ Projects []models.Project }{Projects: list})
}

// SaveProjects — обрабатывает POST из вкладки «Проекты»
func SaveProjects(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	// Перебираем все проекты из БД, чтобы знать id
	rows, _ := db.Pool.Query(context.Background(), "SELECT id FROM projects")
	defer rows.Close()

	for rows.Next() {
		var id int
		rows.Scan(&id)
		// чек-окс
		enabled := r.FormValue(fmt.Sprintf("enabled_%d", id)) == "on"
		custom := r.FormValue(fmt.Sprintf("custom_%d", id))
		db.Pool.Exec(context.Background(),
			"UPDATE projects SET enabled=$1, custom_url=$2 WHERE id=$3",
			enabled, custom, id)
	}

	http.Redirect(w, r, "/admin?tab=projects", 303)
}
