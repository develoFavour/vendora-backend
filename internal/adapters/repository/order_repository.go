package repository

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/developia-II/ecommerce-backend/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type OrderRepository interface {
	PlaceOrder(ctx context.Context, userID primitive.ObjectID, input models.PlaceOrderInput, cart models.Cart) (models.Order, error)
	GetOrdersByUserID(ctx context.Context, userID primitive.ObjectID) ([]models.Order, error)
	GetOrderById(ctx context.Context, orderID primitive.ObjectID) (models.Order, error)
	GetOrdersByVendorID(ctx context.Context, vendorID primitive.ObjectID) ([]models.Order, error)
	UpdateOrderStatus(ctx context.Context, orderID primitive.ObjectID, status models.OrderStatus, trackingNumber string) error
}

type MongoOrderRepository struct {
	DB *mongo.Database
}

func NewOrderRepository(db *mongo.Database) OrderRepository {
	return &MongoOrderRepository{DB: db}
}

func (r *MongoOrderRepository) PlaceOrder(ctx context.Context, userID primitive.ObjectID, input models.PlaceOrderInput, cart models.Cart) (models.Order, error) {
	var err error
	if len(cart.Items) == 0 {
		return models.Order{}, fmt.Errorf("cart is empty")
	}

	productColl := r.DB.Collection("products")
	orderColl := r.DB.Collection("orders")
	cartColl := r.DB.Collection("carts")

	var orderItems []models.OrderItem
	var subtotal float64
	var processedProducts []struct {
		ID  primitive.ObjectID
		Qty int
	}

	// 1. Process each item: Validate Stock and Reduce Inventory
	for _, item := range cart.Items {
		var product models.Product
		err := productColl.FindOne(ctx, bson.M{"_id": item.ProductID}).Decode(&product)
		if err != nil {
			return models.Order{}, fmt.Errorf("product %s not found", item.Name)
		}

		// Atomic update with stock check
		filter := bson.M{
			"_id":   item.ProductID,
			"stock": bson.M{"$gte": item.Quantity},
		}
		update := bson.M{
			"$inc": bson.M{"stock": -item.Quantity},
			"$set": bson.M{"updatedAt": time.Now()},
		}

		res, err := productColl.UpdateOne(ctx, filter, update)
		if err != nil {
			// Rollback previously processed items
			for _, p := range processedProducts {
				productColl.UpdateOne(ctx, bson.M{"_id": p.ID}, bson.M{"$inc": bson.M{"stock": p.Qty}})
			}
			return models.Order{}, err
		}
		if res.ModifiedCount == 0 {
			// Rollback previously processed items
			for _, p := range processedProducts {
				productColl.UpdateOne(ctx, bson.M{"_id": p.ID}, bson.M{"$inc": bson.M{"stock": p.Qty}})
			}
			return models.Order{}, fmt.Errorf("insufficient stock for %s", item.Name)
		}

		processedProducts = append(processedProducts, struct {
			ID  primitive.ObjectID
			Qty int
		}{item.ProductID, item.Quantity})

		itemSubtotal := product.Price * float64(item.Quantity)
		orderItems = append(orderItems, models.OrderItem{
			ProductID: item.ProductID,
			VendorID:  product.VendorID,
			Name:      product.Name,
			Image:     item.Image,
			Price:     product.Price,
			Quantity:  item.Quantity,
			Subtotal:  itemSubtotal,
		})
		subtotal += itemSubtotal
	}

	// 2. Calculate Totals
	shippingFee := 25.0
	if subtotal > 500 {
		shippingFee = 0
	}
	tax := subtotal * 0.05
	total := subtotal + shippingFee + tax

	// 3. Create Order Object
	orderNumber := fmt.Sprintf("VEN-%d%d", time.Now().Unix()%100000, rand.Intn(900)+100)
	order := models.Order{
		ID:              primitive.NewObjectID(),
		OrderNumber:     orderNumber,
		UserID:          userID,
		Items:           orderItems,
		Subtotal:        subtotal,
		ShippingFee:     shippingFee,
		Tax:             tax,
		Total:           total,
		Status:          models.StatusPending,
		PaymentStatus:   "pending",
		PaymentMethod:   input.PaymentMethod,
		ShippingAddress: input.ShippingAddress,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// 4. Insert Order
	ctxInsert, cancelInsert := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelInsert()

	_, err = orderColl.InsertOne(ctxInsert, order)
	if err != nil {
		fmt.Printf("Order creation failed: %v. Rolling back stock.\n", err)
		// Rollback all stock
		for _, p := range processedProducts {
			orderColl.Database().Collection("products").UpdateOne(context.Background(), bson.M{"_id": p.ID}, bson.M{"$inc": bson.M{"stock": p.Qty}})
		}
		return models.Order{}, err
	}

	fmt.Printf("Order %s successfully created and saved to DB\n", order.OrderNumber)

	// 5. Clear Cart (Non-critical lookup)
	_, _ = cartColl.UpdateOne(ctx, bson.M{"userId": userID}, bson.M{"$set": bson.M{"items": []models.CartItem{}, "updatedAt": time.Now()}})

	return order, nil
}

func (r *MongoOrderRepository) GetOrdersByUserID(ctx context.Context, userID primitive.ObjectID) ([]models.Order, error) {
	collection := r.DB.Collection("orders")
	cursor, err := collection.Find(ctx, bson.M{"userId": userID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var orders []models.Order
	if err := cursor.All(ctx, &orders); err != nil {
		return nil, err
	}
	return orders, nil
}

func (r *MongoOrderRepository) GetOrderById(ctx context.Context, orderID primitive.ObjectID) (models.Order, error) {
	collection := r.DB.Collection("orders")
	var order models.Order
	err := collection.FindOne(ctx, bson.M{"_id": orderID}).Decode(&order)
	return order, err
}

func (r *MongoOrderRepository) GetOrdersByVendorID(ctx context.Context, vendorID primitive.ObjectID) ([]models.Order, error) {
	collection := r.DB.Collection("orders")
	// Find orders where at least one item belongs to this vendor
	cursor, err := collection.Find(ctx, bson.M{"items.vendorId": vendorID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var orders []models.Order
	if err := cursor.All(ctx, &orders); err != nil {
		return nil, err
	}
	return orders, nil
}

func (r *MongoOrderRepository) UpdateOrderStatus(ctx context.Context, orderID primitive.ObjectID, status models.OrderStatus, trackingNumber string) error {
	collection := r.DB.Collection("orders")

	updateData := bson.M{
		"status":    status,
		"updatedAt": time.Now(),
	}

	if trackingNumber != "" {
		updateData["trackingNumber"] = trackingNumber
	}

	_, err := collection.UpdateOne(ctx,
		bson.M{"_id": orderID},
		bson.M{"$set": updateData})
	return err
}
