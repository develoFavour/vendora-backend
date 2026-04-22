package database

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ConnectToDB() (*mongo.Database, error) {
	if err := godotenv.Load(); err != nil {
		logrus.Info("No .env file found (using environment variables)")
	}

	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		return nil, fmt.Errorf("MONGO_URI is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logrus.Info("Connecting to MongoDB...")
	client, err := mongo.Connect(ctx, options.Client().
		SetMaxPoolSize(10).
		SetMinPoolSize(5).
		SetMaxConnIdleTime(30*time.Second).
		SetServerSelectionTimeout(10*time.Second).
		SetConnectTimeout(10*time.Second).
		ApplyURI(mongoURI))
	if err != nil {
		return nil, fmt.Errorf("mongo connect failed: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("mongo ping failed: %w", err)
	}

	logrus.Info("MongoDB connection ready")
	return client.Database("vendora"), nil
}
