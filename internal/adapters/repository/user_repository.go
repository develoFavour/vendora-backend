package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/developia-II/ecommerce-backend/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

type UserRepository interface {
	GetByID(ctx context.Context, id primitive.ObjectID) (models.User, error)
	UpdateProfile(ctx context.Context, id primitive.ObjectID, input models.UpdateProfileInput) error
	ChangePassword(ctx context.Context, id primitive.ObjectID, newPassword string) error
	UpdateTier(ctx context.Context, id primitive.ObjectID, tier string) error
	UpdateVendorAppeal(ctx context.Context, id primitive.ObjectID, status string, reason string) error
	UpdateVendorSuspension(ctx context.Context, id primitive.ObjectID, retries int, suspendUntil *time.Time) error
	ListVendorsPublic(ctx context.Context, filter bson.M, limit, skip int) ([]models.User, int64, error)
	FetchVendorPublic(ctx context.Context, filter bson.M) (models.User, error)
}

type MongoUserRepository struct {
	DB *mongo.Database
}

func NewUserRepository(db *mongo.Database) UserRepository {
	return &MongoUserRepository{DB: db}
}

func (r *MongoUserRepository) GetByID(ctx context.Context, id primitive.ObjectID) (models.User, error) {
	collection := r.DB.Collection("users")
	var user models.User
	err := collection.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	return user, err
}

func (r *MongoUserRepository) UpdateProfile(ctx context.Context, id primitive.ObjectID, input models.UpdateProfileInput) error {
	collection := r.DB.Collection("users")

	update := bson.M{
		"$set": bson.M{
			"name":      input.Name,
			"phone":     input.Phone,
			"address":   input.Address,
			"updatedAt": time.Now(),
		},
	}

	// Update nested profile fields
	profileUpdate := bson.M{
		"location":     input.Location,
		"bio":          input.Bio,
		"profileImage": input.ProfilePicture,
	}

	update["$set"].(bson.M)["profile"] = profileUpdate

	res, err := collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return err
	}

	if res.MatchedCount == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

func (r *MongoUserRepository) ChangePassword(ctx context.Context, id primitive.ObjectID, newPassword string) error {
	collection := r.DB.Collection("users")

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	update := bson.M{
		"$set": bson.M{
			"password":  string(hashedPassword),
			"updatedAt": time.Now(),
		},
	}

	res, err := collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return err
	}

	if res.MatchedCount == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

func (r *MongoUserRepository) UpdateTier(ctx context.Context, id primitive.ObjectID, tier string) error {
	collection := r.DB.Collection("users")

	update := bson.M{
		"$set": bson.M{
			"vendorAccount.tier": tier,
			"updatedAt":          time.Now(),
		},
	}

	res, err := collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return err
	}

	if res.MatchedCount == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

func (r *MongoUserRepository) UpdateVendorAppeal(ctx context.Context, id primitive.ObjectID, status string, reason string) error {
	collection := r.DB.Collection("vendorAccounts")
	update := bson.M{
		"$set": bson.M{
			"appealStatus": status,
			"updatedAt":    time.Now(),
		},
	}
	if reason != "" {
		update["$set"].(bson.M)["appealReason"] = reason
	}

	res, err := collection.UpdateOne(ctx, bson.M{"userID": id}, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("vendor account not found")
	}
	return nil
}

func (r *MongoUserRepository) UpdateVendorSuspension(ctx context.Context, id primitive.ObjectID, retries int, suspendUntil *time.Time) error {
	collection := r.DB.Collection("vendorAccounts")
	setFields := bson.M{
		"verificationRetries": retries,
		"updatedAt":           time.Now(),
	}
	if suspendUntil != nil {
		setFields["status"] = "suspended"
		setFields["suspendedUntil"] = suspendUntil
	}
	update := bson.M{"$set": setFields}
	_, err := collection.UpdateOne(ctx, bson.M{"userID": id}, update)
	return err
}

func (r *MongoUserRepository) ListVendorsPublic(ctx context.Context, filter bson.M, limit, skip int) ([]models.User, int64, error) {
	collection := r.DB.Collection("users")

	pipeline := []bson.M{
		{"$match": filter},
		{"$lookup": bson.M{
			"from": "products",
			"let":  bson.M{"vendor_id": "$_id"},
			"pipeline": []bson.M{
				{"$match": bson.M{
					"$expr": bson.M{
						"$and": []bson.M{
							{"$eq": []interface{}{"$vendorId", "$$vendor_id"}},
							{"$eq": []interface{}{"$status", "active"}},
						},
					},
				}},
				{"$sort": bson.M{"createdAt": -1}},
				{"$limit": 4},
			},
			"as": "featuredProducts",
		}},
		{"$sort": bson.M{"createdAt": -1}},
		{"$skip": int64(skip)},
		{"$limit": int64(limit)},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var users []models.User
	if err := cursor.All(ctx, &users); err != nil {
		return nil, 0, err
	}

	total, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func (r *MongoUserRepository) FetchVendorPublic(ctx context.Context, filter bson.M) (models.User, error) {
	collection := r.DB.Collection("users")

	pipeline := []bson.M{
		{"$match": filter},
		{"$lookup": bson.M{
			"from": "products",
			"let":  bson.M{"vendor_id": "$_id"},
			"pipeline": []bson.M{
				{"$match": bson.M{
					"$expr": bson.M{
						"$and": []bson.M{
							{"$eq": []interface{}{"$vendorId", "$$vendor_id"}},
							{"$eq": []interface{}{"$status", "active"}},
						},
					},
				}},
				{"$sort": bson.M{"createdAt": -1}},
			},
			"as": "featuredProducts",
		}},
		{"$limit": 1},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return models.User{}, err
	}
	defer cursor.Close(ctx)

	var users []models.User
	if err := cursor.All(ctx, &users); err != nil {
		return models.User{}, err
	}

	if len(users) == 0 {
		return models.User{}, mongo.ErrNoDocuments
	}

	return users[0], nil
}
