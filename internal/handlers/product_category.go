package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/developia-II/ecommerce-backend/internal/models"
	"github.com/developia-II/ecommerce-backend/utils"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type CategoryHandler struct {
	DB *mongo.Database
}

func NewCategoryHandler(db *mongo.Database) *CategoryHandler {
	return &CategoryHandler{DB: db}
}
func (h *CategoryHandler) CreateProductCategory(c *gin.Context) {
	// Role check is handled by RoleMiddleware in routes.go
	var category models.Category
	if err := c.ShouldBindJSON(&category); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid json payload"))
		return
	}
	if err := validate.Struct(category); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid json payload"))
		return
	}
	category.CreatedAt = time.Now()
	collection := h.DB.Collection("categories")
	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second*10)
	defer cancel()
	var existingCategory models.Category
	if err := collection.FindOne(ctx, bson.M{"slug": category.Slug}).Decode(&existingCategory); err == nil {
		c.JSON(http.StatusConflict, utils.ErrorResponse("category already exists"))
		return
	}
	ctg, err := collection.InsertOne(ctx, category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("failed to create category"))
		return
	}
	res := gin.H{
		"id":           ctg.InsertedID,
		"categoryName": category.Name,
		"createdAt":    category.CreatedAt,
	}
	c.JSON(http.StatusCreated, utils.SuccessResponse("category created successfully", res))
}
func (h *CategoryHandler) GetAllProductCategories(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	limit := 10
	sortOrder := 1
	collection := h.DB.Collection("categories")
	filter := bson.M{}
	opts := options.Find().SetLimit(int64(limit)).SetSort(bson.M{"name": sortOrder})

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("failed to fetch categories"))

		return
	}
	categories := []models.Category{}
	if err := cursor.All(ctx, &categories); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("failed to fetch categories"))
		return
	}
	res := gin.H{
		"categories": categories,
	}
	c.JSON(http.StatusOK, utils.SuccessResponse("categories fetched successfully", res))
}
