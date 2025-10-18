package main

import (
	"Site/db"
	"Site/handlers"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

func main() {
	// Инициализация и отложенное закрытие пула соединений с БД
	db.InitPool()
	defer db.ClosePool()

	// Создаём основной роутер
	r := mux.NewRouter()

	// Статика для загруженных картинок (если используете uploads)
	r.PathPrefix("/uploads/").Handler(
		http.StripPrefix("/uploads/", http.FileServer(http.Dir("static/uploads"))),
	)

	// Публичные маршруты
	r.HandleFunc("/", handlers.Index).Methods("GET")
	r.HandleFunc("/projects", handlers.ProjectsPage).Methods("GET")
	r.HandleFunc("/login", handlers.LoginHandler).Methods("GET", "POST")

	// Админ-панель: все под /admin/*
	admin := r.PathPrefix("/admin").Subrouter()
	admin.Use(handlers.RequireAdmin)

	// GET /admin?tab=posts или /admin?tab=projects
	admin.HandleFunc("", handlers.AdminDashboard).Methods("GET")

	// CRUD для постов
	admin.HandleFunc("/save_post", handlers.SavePost).Methods("POST")
	admin.HandleFunc("/delete_post", handlers.DeletePost).Methods("POST")
	admin.HandleFunc("/update_post", handlers.UpdatePost).Methods("POST")

	// Парсинг и сохранение проектов
	admin.HandleFunc("/projects/refresh", handlers.RefreshProjects).Methods("POST")
	admin.HandleFunc("/projects/save", handlers.SaveProjects).Methods("POST")

	// Запускаем сервер
	log.Println("Server listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
