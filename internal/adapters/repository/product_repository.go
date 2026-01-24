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

type ProductRepository interface {
	FetchProductsPublic(ctx context.Context, filter bson.M, limit, skip int) ([]models.Product, int64, error)
	FetchProductsPublicById(ctx context.Context, filter bson.M) (models.Product, error)
	CreateProduct(ctx context.Context, product models.Product) (models.Product, error)
	GetVendorProducts(ctx context.Context, filter bson.M, limit, skip int64) ([]models.Product, int64, error)
	UpdateProduct(ctx context.Context, filter bson.M, update models.UpdateProductInput) (bool, error)
	GetProduct(ctx context.Context, filter bson.M) (models.Product, error)
	DeleteProduct(ctx context.Context, productID primitive.ObjectID, vendorID primitive.ObjectID) error
}

type MongoProductRepository struct {
	DB *mongo.Database
}

func NewProductRepository(db *mongo.Database) ProductRepository {
	return &MongoProductRepository{DB: db}
}

func (r *MongoProductRepository) FetchProductsPublic(ctx context.Context, filter bson.M, limit, skip int) ([]models.Product, int64, error) {

	collection := r.DB.Collection("products")
	opts := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetSort(bson.M{"createdAt": -1}).
		SetProjection(bson.M{"costPrice": 0})

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var products []models.Product
	if err := cursor.All(ctx, &products); err != nil {
		return nil, 0, err
	}
	total, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return products, total, nil
}
func (r *MongoProductRepository) FetchProductsPublicById(ctx context.Context, filter bson.M) (models.Product, error) {
	collection := r.DB.Collection("products")
	opts := options.FindOne().SetProjection(bson.M{"costPrice": 0})
	var product models.Product
	if err := collection.FindOne(ctx, filter, opts).Decode(&product); err != nil {
		return models.Product{}, err
	}
	return product, nil
}

func (r *MongoProductRepository) CreateProduct(ctx context.Context, product models.Product) (models.Product, error) {
	session, err := r.DB.Client().StartSession()
	if err != nil {
		return models.Product{}, fmt.Errorf("failed to start transaction: %v", err)
	}
	defer session.EndSession(ctx)

	callback := func(sessCtx mongo.SessionContext) (interface{}, error) {
		// 1. Insert product
		collection := r.DB.Collection("products")
		res, err := collection.InsertOne(sessCtx, product)
		if err != nil {
			return nil, err
		}
		product.ID = res.InsertedID.(primitive.ObjectID)

		// 2. Update vendor count atomically
		vendorColl := r.DB.Collection("vendorAccounts")
		update := bson.M{
			"$inc": bson.M{"productCount": 1},
			"$set": bson.M{"updatedAt": time.Now()},
		}
		_, err = vendorColl.UpdateOne(sessCtx, bson.M{"userID": product.VendorID}, update)
		if err != nil {
			return nil, err
		}

		return product, nil
	}

	result, err := session.WithTransaction(ctx, callback)
	if err != nil {
		return models.Product{}, err
	}

	return result.(models.Product), nil
}

func (r *MongoProductRepository) GetVendorProducts(ctx context.Context, filter bson.M, limit, skip int64) ([]models.Product, int64, error) {
	collection := r.DB.Collection("products")
	opts := options.Find().SetSkip(skip).SetLimit(limit).SetSort(bson.M{"createdAt": -1})

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var products []models.Product
	if err := cursor.All(ctx, &products); err != nil {
		return nil, 0, err
	}

	total, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return products, total, nil
}

func (r *MongoProductRepository) UpdateProduct(ctx context.Context, filter bson.M, input models.UpdateProductInput) (bool, error) {
	collection := r.DB.Collection("products")
	update := bson.M{"$set": input}

	result, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return false, err
	}

	return result.MatchedCount > 0, nil
}

func (r *MongoProductRepository) GetProduct(ctx context.Context, filter bson.M) (models.Product, error) {
	collection := r.DB.Collection("products")
	var product models.Product
	if err := collection.FindOne(ctx, filter).Decode(&product); err != nil {
		return models.Product{}, err
	}
	return product, nil
}

func (r *MongoProductRepository) DeleteProduct(ctx context.Context, productID primitive.ObjectID, vendorID primitive.ObjectID) error {
	session, err := r.DB.Client().StartSession()
	if err != nil {
		return fmt.Errorf("failed to start session: %v", err)
	}
	defer session.EndSession(ctx)

	callback := func(sessCtx mongo.SessionContext) (interface{}, error) {
		collection := r.DB.Collection("products")
		filter := bson.M{"_id": productID, "vendorId": vendorID}
		res, err := collection.DeleteOne(sessCtx, filter)
		if err != nil {
			return nil, err
		}
		if res.DeletedCount == 0 {
			return nil, fmt.Errorf("product not found or unauthorized")
		}

		vendorAcc := r.DB.Collection("vendorAccounts")
		_, err = vendorAcc.UpdateOne(sessCtx, bson.M{"userID": vendorID}, bson.M{"$inc": bson.M{"productCount": -1}})
		if err != nil {
			return nil, err
		}
		return nil, nil
	}

	_, err = session.WithTransaction(ctx, callback)
	return err
}
