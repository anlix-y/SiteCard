// handlers/projects.go
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"Site/db"
	"Site/logger"
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

// getNextGitHubToken returns next enabled token by least recently used strategy.
func getNextGitHubToken(ctx context.Context) (string, error) {
	var token string
	err := db.Pool.QueryRow(ctx, `
        SELECT token
          FROM github_tokens
         WHERE enabled = true
         ORDER BY COALESCE(last_used_at, to_timestamp(0)) ASC, id ASC
         LIMIT 1`).Scan(&token)
	if err != nil {
		return "", err
	}
	// mark as used now
	_, _ = db.Pool.Exec(ctx, `UPDATE github_tokens SET last_used_at = NOW() WHERE token = $1`, token)
	return token, nil
}

func disableGitHubToken(ctx context.Context, token string) {
	_, _ = db.Pool.Exec(ctx, `UPDATE github_tokens SET enabled=false, fail_count = fail_count + 1 WHERE token=$1`, token)
}

func RefreshProjects(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	raw := strings.TrimSpace(r.FormValue("github_user"))
	if raw != "" {
		// Если это URL, извлекаем путь после github.com/
		if u, err := url.Parse(raw); err == nil && strings.Contains(u.Host, "github.com") {
			raw = strings.Trim(u.Path, "/")
		}

		parts := strings.Split(raw, "/")
		var apiURL string
		switch len(parts) {
		case 1:
			apiURL = fmt.Sprintf("https://api.github.com/users/%s/repos", parts[0])
		case 2:
			apiURL = fmt.Sprintf("https://api.github.com/repos/%s/%s", parts[0], parts[1])
		default:
			logger.Errorf("RefreshProjects: bad github path: %s", raw)
			http.Error(w, "Неверный формат GitHub-пути", http.StatusBadRequest)
			return
		}

		// Настройка HTTP-клиента и, при необходимости, прокси из окружения (GITHUB_PROXY)
		var client *http.Client
		if proxy := os.Getenv("GITHUB_PROXY"); strings.TrimSpace(proxy) != "" {
			if proxyURL, errp := url.Parse(proxy); errp == nil {
				transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
				client = &http.Client{Transport: transport, Timeout: 30 * time.Second}
			}
		}
		if client == nil {
			client = &http.Client{Timeout: 30 * time.Second}
		}

		// Try with token pool; if none available or all fail, fall back to unauthenticated
		var resp *http.Response
		var err error
		tried := 0
		maxTries := 10
		for tried = 0; tried < maxTries; tried++ {
			req, _ := http.NewRequest("GET", apiURL, nil)
			req.Header.Set("Accept", "application/vnd.github.v3+json")

			token, terr := getNextGitHubToken(r.Context())
			if terr == nil && token != "" {
				req.Header.Set("Authorization", "token "+token)
				resp, err = client.Do(req)
				if err != nil {
					// network error, try next token
					continue
				}
				// Check rate limit headers and statuses
				if resp.StatusCode == http.StatusUnauthorized { // 401
					disableGitHubToken(r.Context(), token)
					resp.Body.Close()
					continue
				}
				if resp.StatusCode == http.StatusForbidden {
					// If rate limit exceeded
					if strings.Contains(strings.ToLower(resp.Status), "forbidden") {
						// try next token; do not disable
						resp.Body.Close()
						continue
					}
				}
				// otherwise accept this response
				break
			} else {
				// No tokens available
				break
			}
		}
		if resp == nil && err == nil {
			// fallback without token
			req, _ := http.NewRequest("GET", apiURL, nil)
			req.Header.Set("Accept", "application/vnd.github.v3+json")
			resp, err = client.Do(req)
		}
		if err != nil {
			logger.Errorf("RefreshProjects: request error: %v", err)
			http.Error(w, "Ошибка запроса к GitHub: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			logger.Errorf("RefreshProjects: GitHub API %d: %s", resp.StatusCode, string(body))
			http.Error(w, fmt.Sprintf("GitHub API %d: %s", resp.StatusCode, body), http.StatusBadGateway)
			return
		}

		// Парсим JSON
		var repos []ghRepo
		if len(parts) == 2 {
			var single ghRepo
			if err := json.NewDecoder(resp.Body).Decode(&single); err != nil {
				logger.Errorf("RefreshProjects: JSON decode (single) error: %v", err)
				http.Error(w, "Ошибка разбора JSON: "+err.Error(), http.StatusBadGateway)
				return
			}
			repos = []ghRepo{single}
		} else {
			if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
				logger.Errorf("RefreshProjects: JSON decode (list) error: %v", err)
				http.Error(w, "Ошибка разбора JSON: "+err.Error(), http.StatusBadGateway)
				return
			}
		}

		// Upsert
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
				logger.Errorf("RefreshProjects: upsert project %s error: %v", repo.Name, err)
				http.Error(w, "DB upsert error for "+repo.Name+": "+err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}

	// Возвращаем актуальную таблицу
	rows, err := db.Pool.Query(context.Background(), `
        SELECT id, repo_name, title, description, image_url,
               github_url, custom_url, enabled
          FROM projects
         ORDER BY updated_at DESC`)
	if err != nil {
		logger.Errorf("RefreshProjects: select projects error: %v", err)
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

	tmpl := template.Must(template.ParseFiles("templates/admin/projects_table.html"))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "projects_table", prjs); err != nil {
		logger.Errorf("RefreshProjects: template render error: %v", err)
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

func SaveProjects(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(10 << 20)

	rows, _ := db.Pool.Query(context.Background(), "SELECT id FROM projects")
	defer rows.Close()

	for rows.Next() {
		var id int
		rows.Scan(&id)

		enabled := r.FormValue(fmt.Sprintf("enabled_%d", id)) == "on"
		custom := r.FormValue(fmt.Sprintf("custom_%d", id))

		imgURL := ""
		file, hdr, err := r.FormFile(fmt.Sprintf("image_%d", id))
		if err == nil {
			defer file.Close()
			fname := fmt.Sprintf("%d_%s", time.Now().Unix(), hdr.Filename)
			dstPath := filepath.Join("static/uploads", fname)
			dst, cerr := os.Create(dstPath)
			if cerr != nil {
				logger.Errorf("SaveProjects: create file error: %v", cerr)
			} else {
				if _, cpyErr := io.Copy(dst, file); cpyErr != nil {
					logger.Errorf("SaveProjects: copy file error: %v", cpyErr)
				}
				dst.Close()
			}
			imgURL = "/uploads/" + fname
		}

		if imgURL != "" {
			if _, err := db.Pool.Exec(context.Background(),
				`UPDATE projects SET enabled=$1, custom_url=$2, image_url=$3 WHERE id=$4`,
				enabled, custom, imgURL, id); err != nil {
				logger.Errorf("SaveProjects: update with image error (id=%d): %v", id, err)
			}
		} else {
			if _, err := db.Pool.Exec(context.Background(),
				`UPDATE projects SET enabled=$1, custom_url=$2 WHERE id=$3`,
				enabled, custom, id); err != nil {
				logger.Errorf("SaveProjects: update without image error (id=%d): %v", id, err)
			}
		}
	}

	// Возвращаем обновлённый кусок таблицы
	rows2, err := db.Pool.Query(context.Background(), `
        SELECT id, repo_name, title, description, image_url,
               github_url, custom_url, enabled
          FROM projects ORDER BY updated_at DESC`)
	if err != nil {
		logger.Errorf("SaveProjects: select projects error: %v", err)
	}
	defer rows2.Close()

	var prjs []models.Project
	for rows2.Next() {
		var p models.Project
		rows2.Scan(&p.ID, &p.RepoName, &p.Title, &p.Description,
			&p.ImageURL, &p.GitHubURL, &p.CustomURL, &p.Enabled)
		prjs = append(prjs, p)
	}

	tmpl := template.Must(template.ParseFiles("templates/admin/projects_table.html"))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.ExecuteTemplate(w, "projects_table", prjs)
}

// ProjectsPage рендерит публичную страницу с включёнными проектами
func ProjectsPage(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Pool.Query(context.Background(), `
        SELECT id, repo_name, title, description, image_url, github_url, custom_url
          FROM projects
         WHERE enabled = true
      ORDER BY updated_at DESC`)
	if err != nil {
		logger.Errorf("ProjectsPage: select enabled projects error: %v", err)
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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.ExecuteTemplate(w, "projects", list)
}
