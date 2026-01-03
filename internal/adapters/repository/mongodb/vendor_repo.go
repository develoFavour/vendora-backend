package mongodb

import (
	"context"
	"errors"
	"time"

	"github.com/developia-II/ecommerce-backend/internal/core/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// VendorRepository implements domain.VendorRepository using MongoDB.
// This struct is an "Adapter" in Hexagonal Architecture terms.
// It adapts our Domain's need for storage to a specific technology (MongoDB).
type VendorRepository struct {
	collection *mongo.Collection
}

// NewVendorRepository creates a new instance of the repository.
// Notice we pass *mongo.Database here, but return the struct pointer.
// In `main.go`, we will assign this to the interface type.
func NewVendorRepository(db *mongo.Database) *VendorRepository {
	return &VendorRepository{
		collection: db.Collection("vendors"),
	}
}

// Create inserts a new vendor application into the database.
func (r *VendorRepository) Create(ctx context.Context, vendor *domain.Vendor) error {
	vendor.CreatedAt = time.Now()
	vendor.UpdatedAt = time.Now()

	// If ID is not set, generate one
	if vendor.ID.IsZero() {
		vendor.ID = primitive.NewObjectID()
	}

	_, err := r.collection.InsertOne(ctx, vendor)
	if err != nil {
		return err
	}

	return nil
}

// GetByUserID finds a vendor application by the user's ID.
func (r *VendorRepository) GetByUserID(ctx context.Context, userID string) (*domain.Vendor, error) {
	objID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, errors.New("invalid user ID format")
	}

	filter := bson.M{"userID": objID}

	var vendor domain.Vendor
	err = r.collection.FindOne(ctx, filter).Decode(&vendor)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // Return nil if not found, let Service handle 404 logic
		}
		return nil, err
	}

	return &vendor, nil
}

// Update modifies an existing vendor application.
func (r *VendorRepository) Update(ctx context.Context, vendor *domain.Vendor) error {
	vendor.UpdatedAt = time.Now()

	filter := bson.M{"_id": vendor.ID}
	update := bson.M{"$set": vendor}

	// ReturnDocument: After ensures we get the updated version if we were returning it,
	// but here we just check for errors.
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	result := r.collection.FindOneAndUpdate(ctx, filter, update, opts)
	if result.Err() != nil {
		return result.Err()
	}

	return nil
}
