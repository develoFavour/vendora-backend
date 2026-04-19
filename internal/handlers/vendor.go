package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/developia-II/ecommerce-backend/internal/adapters/repository"
	"github.com/developia-II/ecommerce-backend/internal/models"
	"github.com/developia-II/ecommerce-backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type VendorHandler struct {
	DB   *mongo.Database
	Repo repository.UserRepository
}

func NewVendorHandler(db *mongo.Database, repo repository.UserRepository) *VendorHandler {
	return &VendorHandler{DB: db, Repo: repo}
}

var vendorValidator = validator.New()

// VendorApplication represents the vendor application data
type VendorApplication struct {
	BusinessName        string   `json:"businessName" validate:"required,min=2,max=100"`
	BusinessType        string   `json:"businessType" validate:"required"`
	BusinessDescription string   `json:"businessDescription" validate:"required,min=10,max=500"`
	ContactEmail        string   `json:"contactEmail" validate:"required,email"`
	ContactPhone        string   `json:"contactPhone" validate:"required,min=10,max=15"`
	BusinessAddress     string   `json:"businessAddress" validate:"required,min=10,max=200"`
	TaxID               string   `json:"taxId,omitempty"`
	Website             string   `json:"website,omitempty"`
	SocialMedia         []string `json:"socialMedia,omitempty"`
	Products            []string `json:"products" validate:"required,min=1"`
	Experience          string   `json:"experience" validate:"required,min=10,max=300"`
	Motivation          string   `json:"motivation" validate:"required,min=10,max=300"`
}

// ApplyForVendor handles vendor application submissions
func (h *VendorHandler) ApplyForVendor(c *gin.Context) {
	// Get user ID from JWT token
	authHeader := c.Request.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("Missing or invalid token"))
		return
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	claims, err := utils.VerifyToken(tokenString)
	if err != nil {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("Invalid or expired token"))
		return
	}

	userID := claims.UserID
	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second*10)
	defer cancel()

	// Get user from database
	collection := h.DB.Collection("users")
	objectId, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid user ID"))
		return
	}

	var user models.User
	filter := bson.M{"_id": objectId}
	if err := collection.FindOne(ctx, filter).Decode(&user); err != nil {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("User not found"))
		return
	}

	// Check if user already has vendor application pending or approved
	if user.VendorStatus == "approved" {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("User is already a verified vendor"))
		return
	}

	if user.VendorStatus == "pending" {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Vendor application already pending review"))
		return
	}

	// Parse vendor application data
	var application VendorApplication
	if err := c.ShouldBindJSON(&application); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid JSON format: "+err.Error()))
		return
	}

	// Validate application data
	if err := vendorValidator.Struct(&application); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Validation failed: "+err.Error()))
		return
	}

	// Create vendor application document
	vendorApp := models.VendorApplication{
		UserID:              objectId,
		BusinessName:        application.BusinessName,
		BusinessType:        application.BusinessType,
		BusinessDescription: application.BusinessDescription,
		ContactEmail:        application.ContactEmail,
		ContactPhone:        application.ContactPhone,
		BusinessAddress:     application.BusinessAddress,
		TaxID:               application.TaxID,
		Website:             application.Website,
		SocialMedia:         application.SocialMedia,
		Products:            application.Products,
		Experience:          application.Experience,
		Motivation:          application.Motivation,
		Status:              "pending",
		AppliedAt:           time.Now(),
		ReviewedAt:          nil,
		ReviewedBy:          "",
		ReviewNotes:         "",
	}

	// Save to vendor_applications collection
	appCollection := h.DB.Collection("vendor_applications")
	_, err = appCollection.InsertOne(ctx, vendorApp)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to submit application"))
		return
	}

	// Update user status to pending
	update := bson.M{
		"$set": bson.M{
			"vendorStatus": "pending",
			"updatedAt":    time.Now(),
		},
	}

	if _, err := collection.UpdateOne(ctx, filter, update); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to update user status"))
		return
	}

	c.JSON(http.StatusCreated, utils.SuccessResponse("Vendor application submitted successfully", gin.H{
		"applicationId": vendorApp.ID.Hex(),
		"status":        "pending",
		"message":       "Your application is under review. You'll be notified once it's approved.",
	}))
}

func (h *VendorHandler) ListPublicVendors(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "12"))
	search := c.Query("search")
	category := c.Query("category")
    
	if page < 1 { page = 1 }
	if limit < 1 { limit = 12 }
	skip := (page - 1) * limit

	filter := bson.M{"vendorStatus": "approved"}
	if search != "" {
		filter["$or"] = []bson.M{
			{"name": bson.M{"$regex": search, "$options": "i"}},
			{"sellerApplication.storeName": bson.M{"$regex": search, "$options": "i"}},
		}
	}
	if category != "" {
		filter["sellerApplication.categories"] = category
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	vendors, total, err := h.Repo.ListVendorsPublic(ctx, filter, limit, skip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to fetch vendors: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Vendors fetched successfully", gin.H{
		"vendors": vendors,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	}))
}

func (h *VendorHandler) GetPublicVendorById(c *gin.Context) {
	id := c.Param("id")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid vendor ID"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	filter := bson.M{"_id": objID, "vendorStatus": "approved"}
	vendor, err := h.Repo.FetchVendorPublic(ctx, filter)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, utils.ErrorResponse("Vendor not found or not yet approved"))
		} else {
			c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to fetch vendor: "+err.Error()))
		}
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Vendor details fetched successfully", vendor))
}
