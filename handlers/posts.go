package handlers

import (
	"Site/db"
	"Site/models"
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
)

func AdminPosts(w http.ResponseWriter, r *http.Request) {
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
		if err := rows.Scan(&p.ID, &p.Label, &p.Text); err != nil {
			log.Println(err)
			continue
		}
		posts = append(posts, p)
	}

	data := AdminViewData{
		ActiveTab: "posts",
		Posts:     posts,
		Edit:      nil,
	}
	tmpl := template.Must(template.ParseFiles("templates/admin/admin.html"))
	tmpl.ExecuteTemplate(w, "admin", data)
}

func SavePost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", 400)
		return
	}
	label, text := r.FormValue("label_post"), r.FormValue("text_post")
	if label == "" || text == "" {
		http.Error(w, "All fields required", 400)
		return
	}
	_, err := db.Pool.Exec(context.Background(),
		"INSERT INTO post(label,text) VALUES($1,$2)", label, text)
	if err != nil {
		fmt.Println("insert error:", err)
		http.Error(w, "DB error", 500)
		return
	}
	http.Redirect(w, r, "/admin", 303)
}

func DeletePost(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.FormValue("id"))
	db.Pool.Exec(context.Background(), "DELETE FROM post WHERE id=$1", id)
	http.Redirect(w, r, "/admin", 303)
}

func EditPost(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.URL.Query().Get("id"))
	var p models.Post
	db.Pool.QueryRow(context.Background(),
		"SELECT id,label,text FROM post WHERE id=$1", id).
		Scan(&p.ID, &p.Label, &p.Text)

	// reload all posts
	rows, _ := db.Pool.Query(context.Background(),
		"SELECT id,label,text FROM post ORDER BY id DESC")
	defer rows.Close()
	var posts []models.Post
	for rows.Next() {
		var q models.Post
		rows.Scan(&q.ID, &q.Label, &q.Text)
		posts = append(posts, q)
	}

	data := AdminViewData{
		ActiveTab: "posts",
		Posts:     posts,
		Edit:      &p,
	}
	tmpl := template.Must(template.ParseFiles("templates/admin/admin.html"))
	tmpl.ExecuteTemplate(w, "admin", data)
}

func UpdatePost(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.FormValue("id"))
	label, text := r.FormValue("label_post"), r.FormValue("text_post")
	db.Pool.Exec(context.Background(),
		"UPDATE post SET label=$1,text=$2 WHERE id=$3", label, text, id)
	http.Redirect(w, r, "/admin", 303)
}
