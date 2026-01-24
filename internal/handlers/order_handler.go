package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/developia-II/ecommerce-backend/internal/adapters/repository"
	"github.com/developia-II/ecommerce-backend/internal/models"
	"github.com/developia-II/ecommerce-backend/utils"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type OrderHandler struct {
	Repo     repository.OrderRepository
	CartRepo repository.CartRepository
}

func NewOrderHandler(db *mongo.Database) *OrderHandler {
	repo := repository.NewOrderRepository(db)
	cartRepo := repository.NewCartRepository(db)
	return &OrderHandler{Repo: repo, CartRepo: cartRepo}
}

// PlaceOrder handles the checkout process
func (h *OrderHandler) PlaceOrder(c *gin.Context) {
	userIdStr, _ := c.Get("userId")
	userID, _ := primitive.ObjectIDFromHex(userIdStr.(string))

	var input models.PlaceOrderInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request body"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 1. Get User's Cart
	cart, err := h.CartRepo.GetCart(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to fetch cart"))
		return
	}

	if len(cart.Items) == 0 {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Your cart is empty"))
		return
	}

	// 2. Place Order (Transaction logic is inside Repo)
	order, err := h.Repo.PlaceOrder(ctx, userID, input, cart)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
		return
	}

	c.JSON(http.StatusCreated, utils.SuccessResponse("Order placed successfully", gin.H{"order": order}))
}

// GetUserOrders returns the order history for a buyer
func (h *OrderHandler) GetUserOrders(c *gin.Context) {
	userIdStr, _ := c.Get("userId")
	userID, _ := primitive.ObjectIDFromHex(userIdStr.(string))

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	orders, err := h.Repo.GetOrdersByUserID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to fetch orders"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Orders fetched successfully", gin.H{"orders": orders}))
}

func (h *OrderHandler) GetOrderById(c *gin.Context) {
	id := c.Param("id")
	orderID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid order ID"))
		return
	}

	userIdStr, _ := c.Get("userId")
	userID, _ := primitive.ObjectIDFromHex(userIdStr.(string))

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	order, err := h.Repo.GetOrderById(ctx, orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("Order not found"))
		return
	}

	if order.UserID != userID {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("You do not have permission to view this order"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Order fetched successfully", gin.H{"order": order}))
}
