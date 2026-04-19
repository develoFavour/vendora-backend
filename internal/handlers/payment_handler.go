package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/developia-II/ecommerce-backend/internal/adapters/repository"
	"github.com/developia-II/ecommerce-backend/internal/models"
	"github.com/developia-II/ecommerce-backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/paymentintent"
	"github.com/stripe/stripe-go/v81/webhook"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type PaymentHandler struct {
	DB              *mongo.Database
	OrderRepo       repository.OrderRepository
	TransactionRepo repository.TransactionRepository
}

func NewPaymentHandler(db *mongo.Database) *PaymentHandler {
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
	orderRepo := repository.NewOrderRepository(db)
	txRepo := repository.NewTransactionRepository(db)
	return &PaymentHandler{
		DB:              db,
		OrderRepo:       orderRepo,
		TransactionRepo: txRepo,
	}
}

func (h *PaymentHandler) CreatePaymentIntent(c *gin.Context) {
	var req struct {
		OrderID string `json:"orderId" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request"))
		return
	}

	orderID, err := primitive.ObjectIDFromHex(req.OrderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid order ID"))
		return
	}

	order, err := h.OrderRepo.GetOrderById(c.Request.Context(), orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("Order not found"))
		return
	}

	amount := int64(order.Total * 100)

	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(amount),
		Currency: stripe.String(string(stripe.CurrencyUSD)),
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
		Metadata: map[string]string{
			"orderId": req.OrderID,
		},
	}

	pi, err := paymentintent.New(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse(fmt.Sprintf("Stripe error: %v", err)))
		return
	}

	// Store PaymentID in order so we can verify if webhook fails
	collection := h.DB.Collection("orders")
	_, _ = collection.UpdateOne(c.Request.Context(),
		bson.M{"_id": orderID},
		bson.M{"$set": bson.M{"paymentId": pi.ID}},
	)

	c.JSON(http.StatusOK, utils.SuccessResponse("Payment intent created", gin.H{
		"clientSecret": pi.ClientSecret,
	}))
}

// VerifyPayment manually checks Stripe status if webhook is missed (e.g. local dev)
func (h *PaymentHandler) VerifyPayment(c *gin.Context) {
	orderIDStr := c.Param("id")
	orderID, err := primitive.ObjectIDFromHex(orderIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid order ID"))
		return
	}

	order, err := h.OrderRepo.GetOrderById(c.Request.Context(), orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("Order not found"))
		return
	}

	if order.PaymentID == "" {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("No payment intent found for this order"))
		return
	}

	// Already paid? Return early
	if order.PaymentStatus == "paid" {
		c.JSON(http.StatusOK, utils.SuccessResponse("Order already paid", nil))
		return
	}

	// Query Stripe directly
	pi, err := paymentintent.Get(order.PaymentID, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Stripe retrieval error"))
		return
	}

	if pi.Status == stripe.PaymentIntentStatusSucceeded {
		// Update DB
		collection := h.DB.Collection("orders")
		_, _ = collection.UpdateOne(c.Request.Context(),
			bson.M{"_id": orderID},
			bson.M{"$set": bson.M{
				"status":        models.StatusPaid,
				"paymentStatus": "paid",
				"updatedAt":     time.Now(),
			}})

		// Credit vendors
		h.creditVendors(c.Request.Context(), order)

		c.JSON(http.StatusOK, utils.SuccessResponse("Payment verified successfully", nil))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Payment status: "+string(pi.Status), nil))
}

func (h *PaymentHandler) creditVendors(ctx context.Context, order models.Order) {
	vendorSales := make(map[primitive.ObjectID]float64)
	for _, item := range order.Items {
		vendorSales[item.VendorID] += item.Subtotal
	}

	for vID, amount := range vendorSales {
		_ = h.TransactionRepo.CreditVendorForSale(ctx, vID, amount, order.ID, order.OrderNumber)
	}
}

// HandleWebhook processes asynchronous events from Stripe
func (h *PaymentHandler) HandleWebhook(c *gin.Context) {

	const MaxBodyBytes = int64(65536)
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxBodyBytes)
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, utils.ErrorResponse("Error reading request body"))
		return
	}

	endpointSecret := strings.TrimSpace(os.Getenv("STRIPE_WEBHOOK_SECRET"))
	if endpointSecret == "" {
		_ = godotenv.Load()
		endpointSecret = strings.TrimSpace(os.Getenv("STRIPE_WEBHOOK_SECRET"))
	}

	signature := c.GetHeader("Stripe-Signature")
	event, err := webhook.ConstructEventWithOptions(payload, signature, endpointSecret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid signature"))
		return
	}

	switch event.Type {
	case "payment_intent.succeeded":
		var pi stripe.PaymentIntent
		err := json.Unmarshal(event.Data.Raw, &pi)
		if err != nil {
			c.JSON(http.StatusBadRequest, utils.ErrorResponse("Error parsing webhook JSON"))
			return
		}

		orderIDStr := pi.Metadata["orderId"]

		if orderIDStr == "" {
			c.JSON(http.StatusOK, gin.H{"success": true})
			return
		}

		orderID, err := primitive.ObjectIDFromHex(orderIDStr)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": true})
			return
		}

		// 3. Mark order as paid in DB
		collection := h.DB.Collection("orders")
		_, err = collection.UpdateOne(c.Request.Context(),
			bson.M{"_id": orderID},
			bson.M{"$set": bson.M{
				"status":        models.StatusPaid,
				"paymentStatus": "paid",
				"paymentId":     pi.ID,
				"updatedAt":     time.Now(),
			}})

		if err != nil {
			c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Database update failed"))
			return
		}

		// 4. Credit Vendors
		order, err := h.OrderRepo.GetOrderById(c.Request.Context(), orderID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Could not retrieve order details from DB"))
			return
		} else {
			h.creditVendors(c.Request.Context(), order)
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
		return

	default:
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}
