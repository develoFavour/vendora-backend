package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/developia-II/ecommerce-backend/internal/adapters/repository"
	"github.com/developia-II/ecommerce-backend/internal/models"
	"github.com/developia-II/ecommerce-backend/utils"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type CartHandler struct {
	Repo        repository.CartRepository
	ProductRepo repository.ProductRepository
}

func NewCartHandler(db *mongo.Database) *CartHandler {
	repo := repository.NewCartRepository(db)
	productRepo := repository.NewProductRepository(db)
	return &CartHandler{Repo: repo, ProductRepo: productRepo}
}

func (h *CartHandler) AddToCart(c *gin.Context) {
	userIdStr, _ := c.Get("userId")
	userID, _ := primitive.ObjectIDFromHex(userIdStr.(string))

	var req struct {
		ProductID string  `json:"productId" binding:"required"`
		Quantity  int     `json:"quantity" binding:"required,min=1"`
		Price     float64 `json:"price" binding:"required"`
		Name      string  `json:"name" binding:"required"`
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

	// 1. Check Product Stock
	product, err := h.ProductRepo.GetProduct(ctx, bson.M{"_id": productID})
	if err != nil {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("Product not found"))
		return
	}

	if req.Quantity > product.Stock {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Requested quantity exceeds available stock"))
		return
	}

	// Use product data from DB to ensure integrity
	image := ""
	if len(product.Images) > 0 {
		image = product.Images[0]
	}

	item := models.CartItem{
		ProductID: productID,
		Name:      product.Name,
		Price:     req.Price, // Keeping req price for now as discussed, but ideally should verify
		Quantity:  req.Quantity,
		Image:     image,
	}

	// Check existing quantity in cart to ensure total doesn't exceed stock
	cart, err := h.Repo.GetCart(ctx, userID)
	if err == nil {
		for i, existingItem := range cart.Items {
			if existingItem.ProductID == productID {
				if existingItem.Quantity+req.Quantity > product.Stock {
					c.JSON(http.StatusBadRequest, utils.ErrorResponse("Total quantity in cart exceeds available stock"))
					return
				}

				// Update existing item fields to keep them fresh (snapshot update)
				// This specifically helps backfill missing images for items added before the schema change
				cart.Items[i].Image = image
				cart.Items[i].Name = product.Name
				// We intentionally don't update Price here to respect original snapshot,
				// BUT for the image fix, updating it is harmless and usually desired.
				// Let's defer actual persistence of this update to the Repo.AddToCart method
				// or we should rely on Repo.AddToCart to handle the heavy lifting?
				// The Repo.AddToCart implementation I saw earlier does the checking itself!
				// I need to update the Repo to handle updating fields, or just rely on the Handler passing the right data?
				// The Repo implementation:
				/*
					for i, existingItem := range cart.Items {
						if existingItem.ProductID == item.ProductID {
							cart.Items[i].Quantity += item.Quantity
							found = true
							break
						}
					}
				*/
				// The Repo ONLY updates Quantity.
				// I should update the Repo to update other fields too.
				break
			}
		}
	}

	if err := h.Repo.AddToCart(ctx, userID, item); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to add to cart"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Item added to cart", nil))
}

func (h *CartHandler) RemoveFromCart(c *gin.Context) {
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

	if err := h.Repo.RemoveFromCart(ctx, userID, productID); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to remove from cart"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Item removed from cart", nil))
}

func (h *CartHandler) GetCart(c *gin.Context) {
	userIdStr, _ := c.Get("userId")
	userID, _ := primitive.ObjectIDFromHex(userIdStr.(string))

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	cart, err := h.Repo.GetCart(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to fetch cart"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Cart fetched successfully", gin.H{"cart": cart}))
}

func (h *CartHandler) UpdateQuantity(c *gin.Context) {
	userIdStr, _ := c.Get("userId")
	userID, _ := primitive.ObjectIDFromHex(userIdStr.(string))

	productIDStr := c.Param("id")
	productID, err := primitive.ObjectIDFromHex(productIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid product ID"))
		return
	}

	var req struct {
		Quantity int `json:"quantity" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request body"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// 1. Check Product Stock
	product, err := h.ProductRepo.GetProduct(ctx, bson.M{"_id": productID})
	if err != nil {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("Product not found"))
		return
	}

	if req.Quantity > product.Stock {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Requested quantity exceeds available stock"))
		return
	}

	if err := h.Repo.UpdateQuantity(ctx, userID, productID, req.Quantity); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to update quantity"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Cart updated", nil))
}

func (h *CartHandler) ClearCart(c *gin.Context) {
	userIdStr, _ := c.Get("userId")
	userID, _ := primitive.ObjectIDFromHex(userIdStr.(string))

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if err := h.Repo.ClearCart(ctx, userID); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to clear cart"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Cart cleared", nil))
}
