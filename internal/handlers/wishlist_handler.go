package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/developia-II/ecommerce-backend/internal/adapters/repository"
	"github.com/developia-II/ecommerce-backend/utils"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type WishlistHandler struct {
	Repo repository.WishlistRepository
}

func NewWishlistHandler(db *mongo.Database) *WishlistHandler {
	repo := repository.NewWishlistRepository(db)
	return &WishlistHandler{Repo: repo}
}

func (h *WishlistHandler) AddToWishlist(c *gin.Context) {
	userIdStr, _ := c.Get("userId")
	userID, _ := primitive.ObjectIDFromHex(userIdStr.(string))

	var req struct {
		ProductID string `json:"productId" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request body"))
		return
	}

	productID, err := primitive.ObjectIDFromHex(req.ProductID)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid product ID"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if err := h.Repo.AddToWishlist(ctx, userID, productID); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to add to wishlist"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Product added to wishlist", nil))
}

func (h *WishlistHandler) RemoveFromWishlist(c *gin.Context) {
	userIdStr, _ := c.Get("userId")
	userID, _ := primitive.ObjectIDFromHex(userIdStr.(string))

	productIDStr := c.Param("id")
	productID, err := primitive.ObjectIDFromHex(productIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid product ID"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if err := h.Repo.RemoveFromWishlist(ctx, userID, productID); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to remove from wishlist"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Product removed from wishlist", nil))
}

func (h *WishlistHandler) GetWishlist(c *gin.Context) {
	userIdStr, _ := c.Get("userId")
	userID, _ := primitive.ObjectIDFromHex(userIdStr.(string))

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	wishlist, err := h.Repo.GetWishlist(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to fetch wishlist"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Wishlist fetched successfully", gin.H{"wishlist": wishlist}))
}
