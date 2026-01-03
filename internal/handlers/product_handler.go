package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/developia-II/ecommerce-backend/internal/models"
	"github.com/developia-II/ecommerce-backend/utils"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ProductHandler struct {
	DB *mongo.Database
}

func NewProductHandler(db *mongo.Database) *ProductHandler {
	return &ProductHandler{DB: db}
}
func (h *ProductHandler) CreateProduct(c *gin.Context) {
	authHeader := c.Request.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("Invalid or missing token"))
		return
	}
	authStr := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := utils.VerifyToken(authStr)
	if err != nil {
		c.JSON(http.StatusForbidden, utils.ErrorResponse(err.Error()))
		return
	}

	if claims.Role != "vendor" {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("Only vendors can create products"))
		return
	}
	userId, err := primitive.ObjectIDFromHex(claims.UserID)
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

	// Start MongoDB transaction for atomic operation
	session, err := h.DB.Client().StartSession()
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to start transaction"))
		return
	}
	defer session.EndSession(ctx)

	// Transaction callback - both operations succeed or both fail
	callback := func(sessCtx mongo.SessionContext) (interface{}, error) {
		// 1. Insert product
		collection := h.DB.Collection("products")
		res, err := collection.InsertOne(sessCtx, product)
		if err != nil {
			return nil, err
		}
		product.ID = res.InsertedID.(primitive.ObjectID)

		// 2. Update vendor count atomically
		vendorColl := h.DB.Collection("vendorAccounts")
		update := bson.M{
			"$inc": bson.M{"productCount": 1},
			"$set": bson.M{"updatedAt": time.Now()},
		}
		_, err = vendorColl.UpdateOne(sessCtx, bson.M{"userID": userId}, update)
		if err != nil {
			return nil, err
		}

		return product, nil
	}

	// Execute transaction with automatic retry on transient errors
	result, err := session.WithTransaction(ctx, callback)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to create product"))
		return
	}

	product = result.(models.Product)
	c.JSON(http.StatusCreated, utils.SuccessResponse("Product created successfully", gin.H{
		"success": true,
		"product": product,
	}))
}

func (h *ProductHandler) GetVendorProducts(c *gin.Context) {
	authHeader := c.Request.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("Invalid or missing token"))
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	authStr := strings.TrimPrefix(authHeader, "Bearer ")
	limit := c.Request.URL.Query().Get("limit")
	page := c.Request.URL.Query().Get("page")
	searchTerm := c.Request.URL.Query().Get("query")
	claims, err := utils.VerifyToken(authStr)
	if err != nil {
		c.JSON(http.StatusForbidden, utils.ErrorResponse(err.Error()))
		return
	}
	if limit == "" {
		limit = "10"
	}
	if page == "" {
		page = "1"
	}
	pageConv, _ := strconv.Atoi(page)
	convLimit, _ := strconv.ParseInt(limit, 10, 64)
	skip := (pageConv - 1) * int(convLimit)
	collection := h.DB.Collection("products")
	vendorID, err := primitive.ObjectIDFromHex(claims.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("invalid or missing token"))
		return
	}
	filter := bson.M{"vendorId": vendorID}

	if searchTerm != "" {
		filter["name"] = bson.M{"$regex": searchTerm, "$options": "i"}
	}
	opts := options.Find().SetSkip(int64(skip)).SetLimit(convLimit).SetSort(bson.M{"createdAt": -1})

	cursor, err := collection.Find(c.Request.Context(), filter, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("failed to fetch products"))
		return
	}
	products := []models.Product{}
	defer cursor.Close(c.Request.Context())
	for cursor.Next(c.Request.Context()) {
		var product models.Product
		if err := cursor.Decode(&product); err != nil {
			c.JSON(http.StatusInternalServerError, utils.ErrorResponse("failed to decode product"))
			return
		}
		products = append(products, product)
	}
	total, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("failed to count documents"))
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

	authHeader := c.Request.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("Invalid or missing token"))
		return
	}
	authStr := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := utils.VerifyToken(authStr)
	if err != nil {
		c.JSON(http.StatusForbidden, utils.ErrorResponse(err.Error()))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	vendorId, err := primitive.ObjectIDFromHex(claims.UserID)
	var existingProduct models.Product

	collections := h.DB.Collection("products")
	if err := collections.FindOne(ctx, bson.M{"_id": productId}).Decode(&existingProduct); err != nil {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("product doesn't exist"))
		return
	}
	if vendorId != existingProduct.VendorID {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("You do not have permission to update this product"))
		return
	}
	filter := bson.M{"vendorId": vendorId, "_id": productId}

	input.UpdatedAt = time.Now()
	update := bson.M{"$set": input}
	result, err := collections.UpdateOne(ctx, filter, update)
	if err != nil {
		fmt.Println(err)
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("failed to update product"))
		return
	}
	if result.MatchedCount == 0 {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("product not found or unauthorized"))
		return
	}
	c.JSON(http.StatusOK, utils.SuccessResponse("product updated successfully", gin.H{"success": true}))
}
