package main

import (
	"context"
	"log"
	"time"

	"siakad-mini/app/repository"
	"siakad-mini/app/service"
	"siakad-mini/config"
	"siakad-mini/database"
	"siakad-mini/helper"
	"siakad-mini/route"
)

func main() {
	config.LoadEnv()

	// 1. Koneksi database
	pool, err := database.NewPool(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	// 2. JWT
	jwtSecret := config.GetEnv("JWT_SECRET", "")
	if len(jwtSecret) < 32 {
		log.Fatal("JWT_SECRET di .env belum diisi atau kurang dari 32 karakter")
	}
	jwtManager := helper.NewJWTManager(
		jwtSecret,
		config.GetEnv("JWT_ISSUER", "siakad-mini"),
		time.Duration(config.GetEnvInt("JWT_ACCESS_TTL_MINUTES", 60))*time.Minute,
	)

	// 3. Repository -> Service
	userRepo := repository.NewUserRepository(pool)
	authService := service.NewAuthService(userRepo, jwtManager)

	// 4. Aplikasi + route
	app := config.NewApp()
	route.Register(app, route.Deps{
		JWT:         jwtManager,
		AuthService: authService,
	})

	port := config.GetEnv("APP_PORT", "3000")
	log.Printf("Server berjalan di http://localhost:%s", port)
	log.Fatal(app.Listen(":" + port))
}
