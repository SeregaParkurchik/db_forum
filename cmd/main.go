package main

import (
	"context"
	"fmt"
	"hardhw/config"
	"hardhw/internal/api"
	"hardhw/internal/routes"
	"hardhw/internal/service"
	"hardhw/internal/storage"
	"log"
	"os"
)

func main() {
	// err := godotenv.Load("../.env")
	// if err != nil {
	// 	log.Fatalf("Ошибка при загрузкие .env файла")
	// }

	ctx := context.Background()
	dbPool, err := config.New(ctx, os.Getenv("PG_DSN"))
	if err != nil {
		log.Fatalf("Ошибка подключения к базе данных: %v", err)
	}
	defer dbPool.Close()

	userStorage := storage.NewPostgresUserStorage(dbPool)
	userService := service.NewUserService(userStorage)
	userHandler := api.NewUserHandler(userService)

	forumStorage := storage.NewPostgresForumStorage(dbPool)
	forumService := service.NewForumService(forumStorage, userStorage)
	forumHandler := api.NewForumHandler(forumService)

	threadStorage := storage.NewPostgresThreadStorage(dbPool)
	threadService := service.NewThreadService(forumStorage, userStorage, threadStorage)
	threadHandler := api.NewThreadHandler(threadService)

	postStorage := storage.NewPostgresPostStorage(dbPool)
	postService := service.NewPostService(forumStorage, userStorage, threadStorage, postStorage)
	postHandler := api.NewPostHandler(postService)

	router := routes.InitRoutes(userHandler, forumHandler, threadHandler, postHandler)

	address, err := config.NewServerAddress()
	if err != nil {
		log.Fatalf("не удалось получить конфигурацию сервера: %v", err)
	}
	fmt.Printf("Запуск HTTP-сервера на адресе: %s\n", address)
	log.Fatal(router.Run(address))

}
