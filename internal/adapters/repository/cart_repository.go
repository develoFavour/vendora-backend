package repository

import (
	"context"
	"time"

	"github.com/developia-II/ecommerce-backend/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type CartRepository interface {
	AddToCart(ctx context.Context, userID primitive.ObjectID, item models.CartItem) error
	RemoveFromCart(ctx context.Context, userID, productID primitive.ObjectID) error
	GetCart(ctx context.Context, userID primitive.ObjectID) (models.Cart, error)
	UpdateQuantity(ctx context.Context, userID, productID primitive.ObjectID, quantity int) error
	ClearCart(ctx context.Context, userID primitive.ObjectID) error
}

type MongoCartRepository struct {
	DB *mongo.Database
}

func NewCartRepository(db *mongo.Database) CartRepository {
	return &MongoCartRepository{DB: db}
}

func (r *MongoCartRepository) AddToCart(ctx context.Context, userID primitive.ObjectID, item models.CartItem) error {
	collection := r.DB.Collection("carts")
	filter := bson.M{"userId": userID}

	// Check if item exists to update quantity, else push new item
	// This is a bit complex in a single query without knowing if it exists.
	// Easier to find -> update or insert.
	// Or use arrayFilters if we want to be fancy, but simple is better for now.

	var cart models.Cart
	err := collection.FindOne(ctx, filter).Decode(&cart)
	if err == mongo.ErrNoDocuments {
		// Create new cart
		cart = models.Cart{
			UserID:    userID,
			Items:     []models.CartItem{item},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		_, err = collection.InsertOne(ctx, cart)
		return err
	} else if err != nil {
		return err
	}

	// Cart exists, check if item exists
	found := false
	for i, existingItem := range cart.Items {
		if existingItem.ProductID == item.ProductID {
			cart.Items[i].Quantity += item.Quantity
			// Update fields to support backfilling/refreshing
			cart.Items[i].Image = item.Image
			cart.Items[i].Name = item.Name
			// We can also update Price if we want the cart to reflect latest price on re-add
			// cart.Items[i].Price = item.Price
			found = true
			break
		}
	}

	if !found {
		cart.Items = append(cart.Items, item)
	}

	cart.UpdatedAt = time.Now()
	_, err = collection.ReplaceOne(ctx, filter, cart)
	return err
}

func (r *MongoCartRepository) RemoveFromCart(ctx context.Context, userID, productID primitive.ObjectID) error {
	collection := r.DB.Collection("carts")
	filter := bson.M{"userId": userID}
	update := bson.M{
		"$pull": bson.M{"items": bson.M{"productId": productID}},
		"$set":  bson.M{"updatedAt": time.Now()},
	}

	_, err := collection.UpdateOne(ctx, filter, update)
	return err
}

func (r *MongoCartRepository) GetCart(ctx context.Context, userID primitive.ObjectID) (models.Cart, error) {
	collection := r.DB.Collection("carts")
	filter := bson.M{"userId": userID}
	var cart models.Cart

	err := collection.FindOne(ctx, filter).Decode(&cart)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return models.Cart{UserID: userID, Items: []models.CartItem{}}, nil
		}
		return models.Cart{}, err
	}
	return cart, nil
}

func (r *MongoCartRepository) UpdateQuantity(ctx context.Context, userID, productID primitive.ObjectID, quantity int) error {
	collection := r.DB.Collection("carts")
	filter := bson.M{"userId": userID, "items.productId": productID}
	update := bson.M{
		"$set": bson.M{
			"items.$.quantity": quantity,
			"updatedAt":        time.Now(),
		},
	}

	_, err := collection.UpdateOne(ctx, filter, update)
	return err
}

func (r *MongoCartRepository) ClearCart(ctx context.Context, userID primitive.ObjectID) error {
	collection := r.DB.Collection("carts")
	filter := bson.M{"userId": userID}
	update := bson.M{
		"$set": bson.M{
			"items":     []models.CartItem{},
			"updatedAt": time.Now(),
		},
	}
	_, err := collection.UpdateOne(ctx, filter, update)
	return err
}
