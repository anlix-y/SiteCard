package handlers

import (
	"Site/db"
	"Site/logger"
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
		logger.Errorf("AdminPosts: load posts error: %v", err)
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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
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
	uid, ok := CurrentUserID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	_, err := db.Pool.Exec(context.Background(),
		"INSERT INTO post(label,text,user_id) VALUES($1,$2,$3)", label, text, uid)
	if err != nil {
		fmt.Println("insert error:", err)
		logger.Errorf("SavePost: insert error (uid=%d): %v", uid, err)
		http.Error(w, "DB error", 500)
		return
	}
	http.Redirect(w, r, "/admin", 303)
}

func DeletePost(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.FormValue("id"))
	uid, ok := CurrentUserID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if _, err := db.Pool.Exec(context.Background(), "DELETE FROM post WHERE id=$1 AND user_id=$2", id, uid); err != nil {
		logger.Errorf("DeletePost: delete error (id=%d, uid=%d): %v", id, uid, err)
	}
	http.Redirect(w, r, "/admin", 303)
}

func EditPost(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.URL.Query().Get("id"))
	var p models.Post
	uid, _ := CurrentUserID(r)
	if err := db.Pool.QueryRow(context.Background(),
		"SELECT id,label,text FROM post WHERE id=$1 AND user_id=$2", id, uid).
		Scan(&p.ID, &p.Label, &p.Text); err != nil {
		logger.Errorf("EditPost: load error (id=%d, uid=%d): %v", id, uid, err)
	}

	// reload all posts
	rows, _ := db.Pool.Query(context.Background(),
		"SELECT id,label,text FROM post WHERE user_id=$1 ORDER BY id DESC", uid)
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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.ExecuteTemplate(w, "admin", data)
}

func UpdatePost(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.FormValue("id"))
	label, text := r.FormValue("label_post"), r.FormValue("text_post")
	uid, ok := CurrentUserID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if _, err := db.Pool.Exec(context.Background(),
		"UPDATE post SET label=$1,text=$2 WHERE id=$3 AND user_id=$4", label, text, id, uid); err != nil {
		logger.Errorf("UpdatePost: update error (id=%d, uid=%d): %v", id, uid, err)
	}
	http.Redirect(w, r, "/admin", 303)
}
