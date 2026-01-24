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

	isBuyer := order.UserID == userID
	isVendor := false
	for _, item := range order.Items {
		if item.VendorID == userID {
			isVendor = true
			break
		}
	}

	if !isBuyer && !isVendor {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("You do not have permission to view this order"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Order fetched successfully", gin.H{"order": order}))
}

func (h *OrderHandler) GetVendorOrders(c *gin.Context) {
	userIdStr, _ := c.Get("userId")
	vendorID, _ := primitive.ObjectIDFromHex(userIdStr.(string))

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	orders, err := h.Repo.GetOrdersByVendorID(ctx, vendorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to fetch vendor orders"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Vendor orders fetched successfully", gin.H{"orders": orders}))
}

func (h *OrderHandler) UpdateVendorOrderStatus(c *gin.Context) {
	id := c.Param("id")
	orderID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid order ID"))
		return
	}

	var input struct {
		Status         models.OrderStatus `json:"status" binding:"required"`
		TrackingNumber string             `json:"trackingNumber"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid status provided"))
		return
	}

	userIdStr, _ := c.Get("userId")
	vendorID, _ := primitive.ObjectIDFromHex(userIdStr.(string))

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// 1. Verify existence and vendor ownership of at least one item
	order, err := h.Repo.GetOrderById(ctx, orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("Order not found"))
		return
	}

	belongsToVendor := false
	for _, item := range order.Items {
		if item.VendorID == vendorID {
			belongsToVendor = true
			break
		}
	}

	if !belongsToVendor {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("You do not have permission to update this order"))
		return
	}

	// 2. Update status
	if err := h.Repo.UpdateOrderStatus(ctx, orderID, input.Status, input.TrackingNumber); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to update status"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Order status updated", nil))
}

func (h *OrderHandler) GetVendorStats(c *gin.Context) {
	userIdStr, _ := c.Get("userId")
	vendorID, _ := primitive.ObjectIDFromHex(userIdStr.(string))

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	stats, err := h.Repo.GetVendorStats(ctx, vendorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to fetch vendor stats"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Vendor stats fetched successfully", gin.H{"stats": stats}))
}

func (h *OrderHandler) ConfirmReceipt(c *gin.Context) {
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

	// 1. Verify existence and buyer ownership
	order, err := h.Repo.GetOrderById(ctx, orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("Order not found"))
		return
	}

	if order.UserID != userID {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("You do not have permission to modify this order"))
		return
	}

	if order.Status != models.StatusShipped {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Only shipped orders can be confirmed"))
		return
	}

	// 2. Update status to Delivered
	if err := h.Repo.UpdateOrderStatus(ctx, orderID, models.StatusDelivered, ""); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to confirm receipt"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Receipt confirmed. Thank you for your acquisition!", nil))
}
