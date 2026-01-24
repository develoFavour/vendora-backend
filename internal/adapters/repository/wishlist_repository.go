package repository

import (
	"context"
	"time"

	"github.com/developia-II/ecommerce-backend/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type WishlistRepository interface {
	AddToWishlist(ctx context.Context, userID, productID primitive.ObjectID) error
	RemoveFromWishlist(ctx context.Context, userID, productID primitive.ObjectID) error
	GetWishlist(ctx context.Context, userID primitive.ObjectID) (models.PopulatedWishlist, error)
}

type MongoWishlistRepository struct {
	DB *mongo.Database
}

func NewWishlistRepository(db *mongo.Database) WishlistRepository {
	return &MongoWishlistRepository{DB: db}
}

func (r *MongoWishlistRepository) AddToWishlist(ctx context.Context, userID, productID primitive.ObjectID) error {
	collection := r.DB.Collection("wishlists")
	filter := bson.M{"userId": userID}
	update := bson.M{
		"$addToSet": bson.M{"productIds": productID},
		"$setOnInsert": bson.M{
			"userId":    userID,
			"createdAt": time.Now(),
		},
	}
	opts := options.Update().SetUpsert(true)

	_, err := collection.UpdateOne(ctx, filter, update, opts)
	return err
}

func (r *MongoWishlistRepository) RemoveFromWishlist(ctx context.Context, userID, productID primitive.ObjectID) error {
	collection := r.DB.Collection("wishlists")
	filter := bson.M{"userId": userID}
	update := bson.M{
		"$pull": bson.M{"productIds": productID},
	}

	_, err := collection.UpdateOne(ctx, filter, update)
	return err
}

func (r *MongoWishlistRepository) GetWishlist(ctx context.Context, userID primitive.ObjectID) (models.PopulatedWishlist, error) {
	collection := r.DB.Collection("wishlists")

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"userId": userID}}},
		{{Key: "$lookup", Value: bson.M{
			"from":         "products",
			"localField":   "productIds",
			"foreignField": "_id",
			"as":           "products",
		}}},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return models.PopulatedWishlist{}, err
	}
	defer cursor.Close(ctx)

	var results []models.PopulatedWishlist
	if err := cursor.All(ctx, &results); err != nil {
		return models.PopulatedWishlist{}, err
	}

	if len(results) == 0 {
		return models.PopulatedWishlist{UserID: userID, Products: []models.Product{}}, nil
	}

	return results[0], nil
}
