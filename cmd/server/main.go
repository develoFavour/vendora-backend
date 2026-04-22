package main

import (
	"os"
	"strings"

	"github.com/developia-II/ecommerce-backend/internal/database"
	"github.com/developia-II/ecommerce-backend/internal/handlers"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

func main() {
	if err := godotenv.Load(); err != nil {
		logrus.Info("No .env file found (using environment variables)")
	}

	logrus.Info("Starting server...")
	logrus.SetLevel(logrus.InfoLevel)
	gin.SetMode(gin.ReleaseMode)

	logrus.Info("Attempting to connect to database...")
	db, err := database.ConnectToDB()
	if err != nil {
		logrus.WithError(err).Warn("Failed to connect to DB - running without database")
		db = nil
	} else {
		logrus.Info("Successfully connected to DB")
	}

	logrus.Info("Setting up Gin router...")
	router := gin.Default()

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "https://vendora-f.vercel.app/"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	logrus.Info("Calling handlers.SetupRoutes...")
	handlers.SetupRoutes(router, db)
	logrus.Info("Routes registered successfully")

	PORT := os.Getenv("PORT")
	if PORT == "" {
		PORT = "8080"
	}

	if !strings.HasPrefix(PORT, ":") {
		PORT = ":" + PORT
	}

	logrus.Info("Starting server on port: " + PORT)
	if err := router.Run(PORT); err != nil {
		logrus.WithError(err).Fatal("Failed to start HTTP server")
	}
}
