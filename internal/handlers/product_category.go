package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/developia-II/ecommerce-backend/internal/models"
	"github.com/developia-II/ecommerce-backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
	var category models.Category
	if err := c.ShouldBindJSON(&category); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid json payload"))
		return
	}

	if err := validate.Struct(category); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Validation failed"))
		return
	}

	if category.Slug == "" {
		category.Slug = utils.GenerateSlug(category.Name)
	}

	category.CreatedAt = time.Now()
	category.UpdatedAt = time.Now()
	category.IsActive = true

	collection := h.DB.Collection("categories")
	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second*10)
	defer cancel()

	logrus.Infof("Attempting to create category: %s (Slug: %s, Parent: %v)", category.Name, category.Slug, category.ParentID)

	var existingCategory models.Category
	if err := collection.FindOne(ctx, bson.M{"slug": category.Slug}).Decode(&existingCategory); err == nil {
		logrus.Warnf("Category creation conflict: slug '%s' already exists (ID: %s)", category.Slug, existingCategory.ID.Hex())
		c.JSON(http.StatusConflict, utils.ErrorResponse("Category with this slug already exists"))
		return
	}

	ctg, err := collection.InsertOne(ctx, category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to create category"))
		return
	}

	res := gin.H{
		"id":        ctg.InsertedID,
		"category":  category,
		"createdAt": category.CreatedAt,
	}
	c.JSON(http.StatusCreated, utils.SuccessResponse("Category created successfully", res))
}

func (h *CategoryHandler) GetAllProductCategories(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Handle query params for filtering
	filter := bson.M{}
	if isActiveStr := c.Query("isActive"); isActiveStr != "" {
		filter["isActive"] = isActiveStr == "true"
	}
	if parentIDStr := c.Query("parentId"); parentIDStr != "" {
		if pID, err := primitive.ObjectIDFromHex(parentIDStr); err == nil {
			filter["parentId"] = pID
		}
	} else if c.Query("topLevel") == "true" {
		filter["parentId"] = nil
	}

	collection := h.DB.Collection("categories")
	opts := options.Find().SetSort(bson.M{"name": 1})

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Failed to fetch categories"))
		return
	}
	defer cursor.Close(ctx)

	categories := []models.Category{}
	if err := cursor.All(ctx, &categories); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to decode categories"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Categories fetched successfully", gin.H{
		"categories": categories,
	}))
}

func (h *CategoryHandler) GetCategoryById(c *gin.Context) {
	idStr := c.Param("id")
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ID format"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	var category models.Category
	if err := h.DB.Collection("categories").FindOne(ctx, bson.M{"_id": id}).Decode(&category); err != nil {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("Category not found"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Category fetched successfully", gin.H{"category": category}))
}

func (h *CategoryHandler) UpdateProductCategory(c *gin.Context) {
	idStr := c.Param("id")
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ID format"))
		return
	}

	var input models.UpdateCategoryInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid update body"))
		return
	}

	logrus.Infof("Updating category: %s with input %+v", idStr, input)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	input.UpdatedAt = time.Now()
	update := bson.M{"$set": input}

	logrus.Infof("Database update filter: _id=%v, update=%+v", id, update)
	res, err := h.DB.Collection("categories").UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		logrus.Errorf("Database error during category update: %v", err)
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to update category"))
		return
	}

	logrus.Infof("Update result: MatchedCount=%d, ModifiedCount=%d", res.MatchedCount, res.ModifiedCount)

	if res.MatchedCount == 0 {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("Category not found"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Category updated successfully", nil))
}

func (h *CategoryHandler) DeleteProductCategory(c *gin.Context) {
	idStr := c.Param("id")
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid ID format"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Check if category has subcategories
	count, _ := h.DB.Collection("categories").CountDocuments(ctx, bson.M{"parentId": id})
	if count > 0 {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Cannot delete category with subcategories. Remove them first."))
		return
	}

	// Check if category has products
	prodCount, _ := h.DB.Collection("products").CountDocuments(ctx, bson.M{"categoryId": id})
	if prodCount > 0 {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Cannot delete category that has products. Reassign them first."))
		return
	}

	res, err := h.DB.Collection("categories").DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to delete category"))
		return
	}

	if res.DeletedCount == 0 {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("Category not found"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Category deleted successfully", nil))
}
