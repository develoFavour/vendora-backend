package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/developia-II/ecommerce-backend/internal/adapters/repository"
	"github.com/developia-II/ecommerce-backend/internal/models"
	"github.com/developia-II/ecommerce-backend/utils"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type ProductHandler struct {
	Repo repository.ProductRepository
	DB   *mongo.Database // Kept for legacy methods until full refactor
}

func NewProductHandler(db *mongo.Database, repo repository.ProductRepository) *ProductHandler {
	return &ProductHandler{
		Repo: repo,
		DB:   db,
	}
}

func (h *ProductHandler) CreateProduct(c *gin.Context) {

	userIdStr, _ := c.Get("userId")
	role, _ := c.Get("role")

	if role.(string) != "vendor" {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("Only vendors can create products"))
		return
	}
	userId, err := primitive.ObjectIDFromHex(userIdStr.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("invalid userId"))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	limitCheck, err := utils.CheckVendorLimits(ctx, userId, h.DB)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"error":      err.Error(),
			"current":    limitCheck.CurrentCount,
			"max":        limitCheck.MaxAllowed,
			"tier":       limitCheck.Tier,
			"upgradeUrl": limitCheck.UpgradeURL,
		})
		return
	}

	var product models.Product
	if err := c.ShouldBindJSON(&product); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid json body"))
		return
	}
	if err := validate.Struct(product); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse(err.Error()))
		return
	}

	product.VendorID = userId
	product.CreatedAt = time.Now()
	product.UpdatedAt = time.Now()

	createdProduct, err := h.Repo.CreateProduct(ctx, product)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to create product"))
		return
	}

	c.JSON(http.StatusCreated, utils.SuccessResponse("Product created successfully", gin.H{
		"success": true,
		"product": createdProduct,
	}))
}

func (h *ProductHandler) GetVendorProducts(c *gin.Context) {
	// Get user info from context (set by AuthMiddleware)
	userIdStr, _ := c.Get("userId")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	limit := c.Request.URL.Query().Get("limit")
	page := c.Request.URL.Query().Get("page")
	searchTerm := c.Request.URL.Query().Get("query")

	if limit == "" {
		limit = "10"
	}
	if page == "" {
		page = "1"
	}
	pageConv, _ := strconv.Atoi(page)
	convLimit, _ := strconv.ParseInt(limit, 10, 64)
	skip := (pageConv - 1) * int(convLimit)

	vendorID, err := primitive.ObjectIDFromHex(userIdStr.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("invalid or missing token"))
		return
	}
	filter := bson.M{"vendorId": vendorID}

	if searchTerm != "" {
		filter["name"] = bson.M{"$regex": searchTerm, "$options": "i"}
	}

	products, total, err := h.Repo.GetVendorProducts(ctx, filter, convLimit, int64(skip))
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("failed to fetch products"))
		return
	}

	res := gin.H{
		"products": products,
		"total":    total,
	}
	c.JSON(http.StatusOK, utils.SuccessResponse("products fetched successfully", res))
}

func (h *ProductHandler) UpdateProduct(c *gin.Context) {
	id, _ := c.Params.Get("id")
	productId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("invalid product id"))
		return
	}

	var input models.UpdateProductInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("invalid json format"))
		return
	}
	if err := validate.Struct(&input); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("invalid json format"))
		return
	}

	// Get user info from context (set by AuthMiddleware)
	userIdStr, _ := c.Get("userId")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	vendorId, err := primitive.ObjectIDFromHex(userIdStr.(string))

	existingProduct, err := h.Repo.GetProduct(ctx, bson.M{"_id": productId})
	if err != nil {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("product doesn't exist"))
		return
	}
	if vendorId != existingProduct.VendorID {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("You do not have permission to update this product"))
		return
	}

	filter := bson.M{"vendorId": vendorId, "_id": productId}
	input.UpdatedAt = time.Now()

	updated, err := h.Repo.UpdateProduct(ctx, filter, input)
	if err != nil {
		fmt.Println(err)
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("failed to update product"))
		return
	}
	if !updated {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("product not found or unauthorized"))
		return
	}
	c.JSON(http.StatusOK, utils.SuccessResponse("product updated successfully", gin.H{"success": true}))
}

func (h *ProductHandler) GetProductById(c *gin.Context) {
	id, _ := c.Params.Get("id")
	productId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("invalid product id"))
		return
	}

	// Get user info from context (set by AuthMiddleware)
	userIdStr, _ := c.Get("userId")
	vendorID, err := primitive.ObjectIDFromHex(userIdStr.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Ensure product belongs to the vendor
	filter := bson.M{"_id": productId, "vendorId": vendorID}
	product, err := h.Repo.GetProduct(ctx, filter)
	if err != nil {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("product not found or unauthorized"))
		return
	}

	res := gin.H{
		"product": product,
	}
	c.JSON(http.StatusOK, utils.SuccessResponse("product fetched successfully", res))

}

func (h *ProductHandler) DeleteProduct(c *gin.Context) {
	id, _ := c.Params.Get("id")
	productId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("invalid product id"))
		return
	}

	// Get user info from context (set by AuthMiddleware)
	userIdStr, _ := c.Get("userId")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	vendorId, err := primitive.ObjectIDFromHex(userIdStr.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("invalid token claims"))
		return
	}

	err = h.Repo.DeleteProduct(ctx, productId, vendorId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("product deleted successfully", gin.H{}))
}

func (h *ProductHandler) FetchProductsPublic(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	searchTerm := c.Query("query")
	category := c.Query("category")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "12"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 12
	}

	filter := h.buildProductFilter(searchTerm, category)
	pageSkip := (page - 1) * limit

	products, total, err := h.Repo.FetchProductsPublic(ctx, filter, limit, pageSkip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("failed to fetch products"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Collection retrieved", gin.H{
		"products": products,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	}))
}
func (h *ProductHandler) FetchProductsPublicById(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("invalid or missing id"))
		return
	}
	productId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("invalid or missing id"))
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	filter := bson.M{"_id": productId, "status": "active"}
	product, err := h.Repo.FetchProductsPublicById(ctx, filter)
	if err != nil {
		c.JSON(http.StatusNotFound, utils.ErrorResponse(err.Error()))
		return
	}

	c.JSON(http.StatusOK, product)

}

func (h *ProductHandler) buildProductFilter(query, category string) bson.M {
	filter := bson.M{
		"status": "active",
	}

	if query != "" {
		filter["$text"] = bson.M{"$search": query}
	}

	if category != "" {

		if catID, err := primitive.ObjectIDFromHex(category); err == nil {
			filter["categoryId"] = catID
		}
	}

	return filter
}
