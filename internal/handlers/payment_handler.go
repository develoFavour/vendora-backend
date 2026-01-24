package handlers

import (
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
	DB        *mongo.Database
	OrderRepo repository.OrderRepository
}

func NewPaymentHandler(db *mongo.Database) *PaymentHandler {
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
	orderRepo := repository.NewOrderRepository(db)
	return &PaymentHandler{DB: db, OrderRepo: orderRepo}
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

	c.JSON(http.StatusOK, utils.SuccessResponse("Payment intent created", gin.H{
		"clientSecret": pi.ClientSecret,
	}))
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

	// Fallback check: if somehow secret is missing, try loading env again
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

		orderID, err := primitive.ObjectIDFromHex(orderIDStr)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": true}) // Return 200 so Stripe doesn't retry invalid data
			return
		}

		// 3. Mark order as paid in DB
		collection := h.DB.Collection("orders")
		updateResult, err := collection.UpdateOne(c.Request.Context(),
			bson.M{"_id": orderID},
			bson.M{"$set": bson.M{
				"status":        models.StatusPaid,
				"paymentStatus": "paid",
				"paymentId":     pi.ID,
				"updatedAt":     time.Now(),
			}})

		if err != nil {
			c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to update order in DB"))
			return
		} else if updateResult.MatchedCount == 0 {
			c.JSON(http.StatusOK, gin.H{"success": true}) // Return 200 so Stripe doesn't retry invalid data
			return
		} else {
			c.JSON(http.StatusOK, gin.H{"success": true}) // Return 200 so Stripe doesn't retry invalid data
			return
		}

	default:
		c.JSON(http.StatusOK, gin.H{"success": true}) // Return 200 so Stripe doesn't retry invalid data
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
