package main

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load(".env")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, _ := mongo.Connect(ctx, options.Client().ApplyURI(os.Getenv("MONGO_URI")))
	defer client.Disconnect(ctx)
	
	cur, _ := client.Database("vendora").Collection("drafts").Find(ctx, bson.M{})
	var drafts []bson.M
	cur.All(ctx, &drafts)
	bytes, _ := json.MarshalIndent(drafts, "", "  ")
	
	os.WriteFile("draft.json", bytes, 0644)
}
