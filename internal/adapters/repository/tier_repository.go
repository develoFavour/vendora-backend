package repository

import (
	"context"

	"github.com/developia-II/ecommerce-backend/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type TierRepository interface {
	CreateUpgradeRequest(ctx context.Context, req models.TierUpgradeRequest) error
	GetPendingUpgradeRequest(ctx context.Context, vendorID primitive.ObjectID) (*models.TierUpgradeRequest, error)
	GetLatestUpgradeRequest(ctx context.Context, vendorID primitive.ObjectID) (*models.TierUpgradeRequest, error)
	GetUpgradeHistory(ctx context.Context, vendorID primitive.ObjectID) ([]models.TierUpgradeRequest, error)
}

type MongoTierRepository struct {
	DB *mongo.Database
}

func NewTierRepository(db *mongo.Database) TierRepository {
	return &MongoTierRepository{DB: db}
}

func (r *MongoTierRepository) CreateUpgradeRequest(ctx context.Context, req models.TierUpgradeRequest) error {
	collection := r.DB.Collection("tierUpgradeRequests")
	_, err := collection.InsertOne(ctx, req)
	return err
}

func (r *MongoTierRepository) GetPendingUpgradeRequest(ctx context.Context, vendorID primitive.ObjectID) (*models.TierUpgradeRequest, error) {
	collection := r.DB.Collection("tierUpgradeRequests")
	var req models.TierUpgradeRequest
	err := collection.FindOne(ctx, bson.M{
		"vendorID": vendorID,
		"status":   models.UpgradeStatusPending,
	}).Decode(&req)

	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &req, err
}

func (r *MongoTierRepository) GetLatestUpgradeRequest(ctx context.Context, vendorID primitive.ObjectID) (*models.TierUpgradeRequest, error) {
	collection := r.DB.Collection("tierUpgradeRequests")

	opts := options.FindOne().SetSort(bson.M{"createdAt": -1})
	var req models.TierUpgradeRequest

	err := collection.FindOne(ctx, bson.M{"vendorID": vendorID}, opts).Decode(&req)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &req, err
}

func (r *MongoTierRepository) GetUpgradeHistory(ctx context.Context, vendorID primitive.ObjectID) ([]models.TierUpgradeRequest, error) {
	collection := r.DB.Collection("tierUpgradeRequests")

	opts := options.Find().SetSort(bson.M{"createdAt": -1})
	cursor, err := collection.Find(ctx, bson.M{"vendorID": vendorID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	history := []models.TierUpgradeRequest{}
	if err := cursor.All(ctx, &history); err != nil {
		return nil, err
	}
	return history, nil
}
