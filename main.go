package main

import (
	"Site/config"
	"Site/db"
	"Site/handlers"
	"Site/logger"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

func main() {
	// Сначала загружаем .env (для совместимости), затем конфиг-файл, который имеет приоритет
	godotenv.Load()
	config.Load()
	// Инициализация логгера из переменных окружения
	logger.Init()
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
	r.HandleFunc("/", handlers.RootHandler).Methods("GET")
	r.HandleFunc("/projects", handlers.ProjectsPage).Methods("GET")
	r.HandleFunc("/login", handlers.LoginHandler).Methods("GET", "POST")
	r.HandleFunc("/register", handlers.RegisterHandler).Methods("GET", "POST")
	r.HandleFunc("/logout", handlers.LogoutHandler).Methods("GET", "POST")

	// Админ-панель (по просьбе — доступна всем авторизованным пользователям)
	admin := r.PathPrefix("/admin").Subrouter()
	admin.Use(handlers.RequireAuth)

	// GET /admin?tab=posts или /admin?tab=projects
	admin.HandleFunc("", handlers.AdminDashboard).Methods("GET")

	// CRUD для постов
	admin.HandleFunc("/save_post", handlers.SavePost).Methods("POST")
	admin.HandleFunc("/delete_post", handlers.DeletePost).Methods("POST")
	admin.HandleFunc("/update_post", handlers.UpdatePost).Methods("POST")
	admin.HandleFunc("/edit_post", handlers.EditPost).Methods("GET")

	// Парсинг и сохранение проектов
	admin.HandleFunc("/projects/refresh", handlers.RefreshProjects).Methods("POST")
	admin.HandleFunc("/projects/save", handlers.SaveProjects).Methods("POST")

	// Личный кабинет для любого залогиненного пользователя
	cabinet := r.PathPrefix("").Subrouter()
	cabinet.Use(handlers.RequireAuth)
	cabinet.HandleFunc("/cabinet", handlers.CabinetPage).Methods("GET")
	cabinet.HandleFunc("/cabinet/save", handlers.CabinetSave).Methods("POST")

	// Публичные персональные страницы по слагу — регистрируем в самом конце,
	// чтобы не перехватить системные пути
	r.HandleFunc("/{slug}", handlers.PublicProfile).Methods("GET")

	// Запускаем сервер
	listen := os.Getenv("LISTEN")
	if listen == "" {
		listen = ":8080"
	}
	if logger.Enabled() {
		logger.Infof("Server starting on %s", listen)
	} else {
		log.Println("Server listening on", listen)
	}
	log.Fatal(http.ListenAndServe(listen, r))
}
