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

type ReviewHandler struct {
	Repo repository.ReviewRepository
}

func NewReviewHandler(db *mongo.Database) *ReviewHandler {
	repo := repository.NewReviewRepository(db)
	return &ReviewHandler{Repo: repo}
}

func (h *ReviewHandler) CreateReview(c *gin.Context) {
	userIdStr, _ := c.Get("userId")
	userID, _ := primitive.ObjectIDFromHex(userIdStr.(string))

	// Get User Info for the review (Name and Profile Image)
	userColl := h.Repo.(*repository.MongoReviewRepository).DB.Collection("users")
	var user models.User
	if err := userColl.FindOne(c.Request.Context(), bson.M{"_id": userID}).Decode(&user); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to fetch user details"))
		return
	}

	userName := user.Name
	userImage := ""
	if user.Profile != nil {
		userImage = user.Profile.ProfilePicture
	}

	var input models.CreateReviewInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request body"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	review, err := h.Repo.CreateReview(ctx, userID, userName, userImage, input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
		return
	}

	c.JSON(http.StatusCreated, utils.SuccessResponse("Review submitted successfully", gin.H{"review": review}))
}

func (h *ReviewHandler) GetProductReviews(c *gin.Context) {
	id := c.Param("id")
	productID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid product ID"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	reviews, err := h.Repo.GetProductReviews(ctx, productID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to fetch reviews"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Reviews fetched successfully", gin.H{"reviews": reviews}))
}

func (h *ReviewHandler) GetVendorReviews(c *gin.Context) {
	userIdStr, _ := c.Get("userId")
	vendorID, _ := primitive.ObjectIDFromHex(userIdStr.(string))

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	reviews, err := h.Repo.GetVendorReviews(ctx, vendorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to fetch vendor reviews"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Vendor reviews fetched successfully", gin.H{"reviews": reviews}))
}

func (h *ReviewHandler) RespondToReview(c *gin.Context) {
	reviewIDStr := c.Param("id")
	reviewID, err := primitive.ObjectIDFromHex(reviewIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid review ID"))
		return
	}

	userIdStr, _ := c.Get("userId")
	vendorID, _ := primitive.ObjectIDFromHex(userIdStr.(string))

	var input models.VendorResponseInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request body"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if err := h.Repo.AddVendorResponse(ctx, reviewID, vendorID, input.Response); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Response added successfully", nil))
}
