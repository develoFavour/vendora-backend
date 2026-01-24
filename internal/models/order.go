package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type OrderStatus string

const (
	StatusPending   OrderStatus = "pending"
	StatusPaid      OrderStatus = "paid"
	StatusConfirmed OrderStatus = "confirmed"
	StatusShipped   OrderStatus = "shipped"
	StatusDelivered OrderStatus = "delivered"
	StatusCancelled OrderStatus = "cancelled"
	StatusRefunded  OrderStatus = "refunded"
)

type OrderItem struct {
	ProductID primitive.ObjectID `json:"productId" bson:"productId"`
	VendorID  primitive.ObjectID `json:"vendorId" bson:"vendorId"`
	Name      string             `json:"name" bson:"name"`
	Image     string             `json:"image" bson:"image"`
	Price     float64            `json:"price" bson:"price"`
	Quantity  int                `json:"quantity" bson:"quantity"`
	Subtotal  float64            `json:"subtotal" bson:"subtotal"`
}

type Order struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	OrderNumber string             `json:"orderNumber" bson:"orderNumber"` // e.g., VEN-100234
	UserID      primitive.ObjectID `json:"userId" bson:"userId"`
	Items       []OrderItem        `json:"items" bson:"items"`

	// Pricing Breakdown
	Subtotal    float64 `json:"subtotal" bson:"subtotal"`
	ShippingFee float64 `json:"shippingFee" bson:"shippingFee"`
	Tax         float64 `json:"tax" bson:"tax"`
	Total       float64 `json:"total" bson:"total"`

	Status        OrderStatus `json:"status" bson:"status"`
	PaymentStatus string      `json:"paymentStatus" bson:"paymentStatus"`
	PaymentID     string      `json:"paymentId" bson:"paymentId"`
	PaymentMethod string      `json:"paymentMethod" bson:"paymentMethod"`

	ShippingAddress string `json:"shippingAddress" bson:"shippingAddress"`
	TrackingNumber  string `json:"trackingNumber" bson:"trackingNumber"`

	CreatedAt time.Time `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt" bson:"updatedAt"`
}

type PlaceOrderInput struct {
	ShippingAddress string `json:"shippingAddress" binding:"required"`
	PaymentMethod   string `json:"paymentMethod" binding:"required"`
}

type DailySales struct {
	Date    string  `json:"date"`
	Revenue float64 `json:"revenue"`
	Orders  int     `json:"orders"`
}

type VendorStats struct {
	TotalRevenue     float64        `json:"totalRevenue"`
	TotalOrders      int            `json:"totalOrders"`
	TotalProducts    int            `json:"totalProducts"`
	AvgOrderValue    float64        `json:"avgOrderValue"`
	SalesPerformance []DailySales   `json:"salesPerformance"`
	StatusBreakdown  map[string]int `json:"statusBreakdown"`
}
