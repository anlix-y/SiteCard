// handlers/projects.go
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"Site/db"
	"Site/models"
)

// ghRepo описывает JSON-ответ GitHub API
type ghRepo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	HTMLURL     string `json:"html_url"`
	Owner       struct {
		AvatarURL string `json:"avatar_url"`
	} `json:"owner"`
}

// AdminProjects рендерит вкладку «Проекты» в админке
func AdminProjects(w http.ResponseWriter, r *http.Request) {
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

	var prjs []models.Project
	for rows.Next() {
		var p models.Project
		rows.Scan(&p.ID, &p.RepoName, &p.Title, &p.Description,
			&p.ImageURL, &p.GitHubURL, &p.CustomURL, &p.Enabled)
		prjs = append(prjs, p)
	}

	data := AdminViewData{
		ActiveTab: "projects",
		Projects:  prjs,
	}
	tmpl := template.Must(template.ParseFiles("templates/admin/admin.html"))
	tmpl.ExecuteTemplate(w, "admin", data)
}

// RefreshProjects подтягивает репозитории из GitHub и сохраняет их в БД
func RefreshProjects(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimSpace(r.FormValue("github_user"))
	if raw == "" {
		// ничего не парсим, просто показываем текущие проекты
		http.Redirect(w, r, "/admin?tab=projects", http.StatusSeeOther)
		return
	}

	// Если это URL, извлекаем путь после github.com/
	if u, err := url.Parse(raw); err == nil && strings.Contains(u.Host, "github.com") {
		raw = strings.Trim(u.Path, "/")
	}

	// Определяем endpoint: user → /users/{user}/repos, или owner/repo → /repos/{owner}/{repo}
	parts := strings.Split(raw, "/")
	var apiURL string
	switch len(parts) {
	case 1:
		apiURL = fmt.Sprintf("https://api.github.com/users/%s/repos", parts[0])
	case 2:
		apiURL = fmt.Sprintf("https://api.github.com/repos/%s/%s", parts[0], parts[1])
	default:
		http.Error(w, "Неверный формат GitHub-пути", http.StatusBadRequest)
		return
	}

	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("GitHub request error: %v\n", err)
		http.Redirect(w, r, "/admin?tab=projects", http.StatusSeeOther)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("GitHub API status %d: %s\n", resp.StatusCode, body)
		http.Redirect(w, r, "/admin?tab=projects", http.StatusSeeOther)
		return
	}

	// Парсим JSON
	var repos []ghRepo
	if len(parts) == 2 {
		var single ghRepo
		if err := json.NewDecoder(resp.Body).Decode(&single); err != nil {
			log.Printf("JSON decode error: %v\n", err)
			http.Redirect(w, r, "/admin?tab=projects", http.StatusSeeOther)
			return
		}
		repos = []ghRepo{single}
	} else {
		if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
			log.Printf("JSON decode error: %v\n", err)
			http.Redirect(w, r, "/admin?tab=projects", http.StatusSeeOther)
			return
		}
	}

	// Upsert в БД
	for _, repo := range repos {
		_, err := db.Pool.Exec(context.Background(), `
            INSERT INTO projects(repo_name,title,description,image_url,github_url,enabled)
            VALUES($1,$2,$3,$4,$5,false)
            ON CONFLICT(repo_name) DO UPDATE SET
              title       = EXCLUDED.title,
              description = EXCLUDED.description,
              image_url   = EXCLUDED.image_url,
              github_url  = EXCLUDED.github_url,
              updated_at  = NOW()
        `, repo.Name, repo.Name, repo.Description, repo.Owner.AvatarURL, repo.HTMLURL)
		if err != nil {
			log.Printf("DB upsert error for %s: %v\n", repo.Name, err)
		}
	}

	http.Redirect(w, r, "/admin?tab=projects", http.StatusSeeOther)
}

// SaveProjects сохраняет флаги enabled, custom_url и загруженные картинки
func SaveProjects(w http.ResponseWriter, r *http.Request) {
	// Для загрузки картинок
	r.ParseMultipartForm(10 << 20) // до 10 МБ

	rows, _ := db.Pool.Query(context.Background(), "SELECT id FROM projects")
	defer rows.Close()

	for rows.Next() {
		var id int
		rows.Scan(&id)

		enabled := r.FormValue(fmt.Sprintf("enabled_%d", id)) == "on"
		custom := r.FormValue(fmt.Sprintf("custom_%d", id))

		// Обработка файла
		imgURL := ""
		file, hdr, err := r.FormFile(fmt.Sprintf("image_%d", id))
		if err == nil {
			defer file.Close()
			fname := fmt.Sprintf("%d_%s", time.Now().Unix(), hdr.Filename)
			dstPath := filepath.Join("static/uploads", fname)
			dst, _ := os.Create(dstPath)
			io.Copy(dst, file)
			dst.Close()
			imgURL = "/uploads/" + fname
		}

		// Обновляем запись
		if imgURL != "" {
			db.Pool.Exec(context.Background(),
				`UPDATE projects
                    SET enabled=$1, custom_url=$2, image_url=$3
                  WHERE id=$4`,
				enabled, custom, imgURL, id)
		} else {
			db.Pool.Exec(context.Background(),
				`UPDATE projects
                    SET enabled=$1, custom_url=$2
                  WHERE id=$3`,
				enabled, custom, id)
		}
	}

	http.Redirect(w, r, "/admin?tab=projects", http.StatusSeeOther)
}

// ProjectsPage рендерит публичную страницу с включёнными проектами
func ProjectsPage(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Pool.Query(context.Background(), `
        SELECT id, repo_name, title, description, image_url, github_url, custom_url
          FROM projects
         WHERE enabled = true
      ORDER BY updated_at DESC`)
	if err != nil {
		http.Error(w, "DB error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var list []models.Project
	for rows.Next() {
		var p models.Project
		rows.Scan(&p.ID, &p.RepoName, &p.Title, &p.Description,
			&p.ImageURL, &p.GitHubURL, &p.CustomURL)
		list = append(list, p)
	}

	tmpl := template.Must(template.ParseFiles(
		"templates/header.html",
		"templates/projects.html",
		"templates/footer.html",
	))
	tmpl.ExecuteTemplate(w, "projects", list)
}
