package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/developia-II/ecommerce-backend/internal/models"
	"github.com/developia-II/ecommerce-backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type VendorHandler struct {
	DB *mongo.Database
}

func NewVendorHandler(db *mongo.Database) *VendorHandler {
	return &VendorHandler{DB: db}
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
