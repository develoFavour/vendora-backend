package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/developia-II/ecommerce-backend/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ReviewRepository interface {
	CreateReview(ctx context.Context, userID primitive.ObjectID, userName string, userImage string, input models.CreateReviewInput) (models.Review, error)
	GetProductReviews(ctx context.Context, productID primitive.ObjectID) ([]models.Review, error)
	GetVendorReviews(ctx context.Context, vendorID primitive.ObjectID) ([]models.Review, error)
	AddVendorResponse(ctx context.Context, reviewID primitive.ObjectID, vendorID primitive.ObjectID, response string) error
	GetAverageRating(ctx context.Context, productID primitive.ObjectID) (float64, int, error)
}

type MongoReviewRepository struct {
	DB *mongo.Database
}

func NewReviewRepository(db *mongo.Database) ReviewRepository {
	return &MongoReviewRepository{DB: db}
}

func (r *MongoReviewRepository) CreateReview(ctx context.Context, userID primitive.ObjectID, userName string, userImage string, input models.CreateReviewInput) (models.Review, error) {
	reviewColl := r.DB.Collection("reviews")
	productColl := r.DB.Collection("products")

	// 1. Get Product to find VendorID
	var product models.Product
	if err := productColl.FindOne(ctx, bson.M{"_id": input.ProductID}).Decode(&product); err != nil {
		return models.Review{}, fmt.Errorf("product not found")
	}

	// 2. Check if user already reviewed this product from this order
	count, _ := reviewColl.CountDocuments(ctx, bson.M{
		"userId":    userID,
		"productId": input.ProductID,
		"orderId":   input.OrderID,
	})
	if count > 0 {
		return models.Review{}, fmt.Errorf("you have already reviewed this product for this order")
	}

	review := models.Review{
		ID:        primitive.NewObjectID(),
		ProductID: input.ProductID,
		OrderID:   input.OrderID,
		UserID:    userID,
		VendorID:  product.VendorID,
		UserName:  userName,
		UserImage: userImage,
		Rating:    input.Rating,
		Comment:   input.Comment,
		Images:    input.Images,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if _, err := reviewColl.InsertOne(ctx, review); err != nil {
		return models.Review{}, err
	}

	// 3. Update Product Aggregate Ratings (simplistic update, in production use aggregation or atomic increments)
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		avg, total, _ := r.GetAverageRating(bgCtx, input.ProductID)
		productColl.UpdateOne(bgCtx, bson.M{"_id": input.ProductID}, bson.M{
			"$set": bson.M{
				"rating":      avg,
				"reviewCount": total,
			},
		})
	}()

	return review, nil
}

func (r *MongoReviewRepository) GetProductReviews(ctx context.Context, productID primitive.ObjectID) ([]models.Review, error) {
	collection := r.DB.Collection("reviews")
	opts := options.Find().SetSort(bson.M{"createdAt": -1})
	cursor, err := collection.Find(ctx, bson.M{"productId": productID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var reviews []models.Review
	if err := cursor.All(ctx, &reviews); err != nil {
		return nil, err
	}
	return reviews, nil
}

func (r *MongoReviewRepository) GetVendorReviews(ctx context.Context, vendorID primitive.ObjectID) ([]models.Review, error) {
	collection := r.DB.Collection("reviews")
	opts := options.Find().SetSort(bson.M{"createdAt": -1})
	cursor, err := collection.Find(ctx, bson.M{"vendorId": vendorID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var reviews []models.Review
	if err := cursor.All(ctx, &reviews); err != nil {
		return nil, err
	}
	return reviews, nil
}

func (r *MongoReviewRepository) AddVendorResponse(ctx context.Context, reviewID primitive.ObjectID, vendorID primitive.ObjectID, response string) error {
	collection := r.DB.Collection("reviews")
	now := time.Now()
	res, err := collection.UpdateOne(ctx,
		bson.M{"_id": reviewID, "vendorId": vendorID},
		bson.M{"$set": bson.M{
			"response":   response,
			"responseAt": &now,
			"updatedAt":  now,
		}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("review not found or you are not the owner")
	}
	return nil
}

func (r *MongoReviewRepository) GetAverageRating(ctx context.Context, productID primitive.ObjectID) (float64, int, error) {
	collection := r.DB.Collection("reviews")
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"productId": productID}}},
		{{Key: "$group", Value: bson.M{
			"_id":       "$productId",
			"avgRating": bson.M{"$avg": "$rating"},
			"total":     bson.M{"$sum": 1},
		}}},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, 0, err
	}
	defer cursor.Close(ctx)

	var results []struct {
		AvgRating float64 `bson:"avgRating"`
		Total     int     `bson:"total"`
	}
	if err := cursor.All(ctx, &results); err != nil {
		return 0, 0, err
	}

	if len(results) == 0 {
		return 0, 0, nil
	}

	return results[0].AvgRating, results[0].Total, nil
}
