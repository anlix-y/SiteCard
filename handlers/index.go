package handlers

import (
	"context"
	"html/template"
	"net/http"
	"unicode/utf8"

	"Site/db"
	"Site/models"
	"github.com/russross/blackfriday/v2"
)

type IndexPageData struct {
	Posts []models.Post
}

func Index(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Pool.Query(context.Background(),
		"SELECT id,label,text FROM post ORDER BY id DESC")
	if err != nil {
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
	tmpl.ExecuteTemplate(w, "index", IndexPageData{Posts: posts})
}
