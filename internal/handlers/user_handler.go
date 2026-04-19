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
	"golang.org/x/crypto/bcrypt"
)

type UserHandler struct {
	Repo repository.UserRepository
}

func NewUserHandler(db *mongo.Database) *UserHandler {
	repo := repository.NewUserRepository(db)
	return &UserHandler{Repo: repo}
}

func (h *UserHandler) GetProfile(c *gin.Context) {
	userIdStr, _ := c.Get("userId")
	userID, _ := primitive.ObjectIDFromHex(userIdStr.(string))

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	user, err := h.Repo.GetByID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("User profile not found"))
		return
	}

	// Mask password
	user.Password = ""

	c.JSON(http.StatusOK, utils.SuccessResponse("Profile fetched successfully", gin.H{"user": user}))
}

func (h *UserHandler) UpdateProfile(c *gin.Context) {
	userIdStr, _ := c.Get("userId")
	userID, _ := primitive.ObjectIDFromHex(userIdStr.(string))

	var input models.UpdateProfileInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request body"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if err := h.Repo.UpdateProfile(ctx, userID, input); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to update profile"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Profile updated successfully", nil))
}

func (h *UserHandler) ChangePassword(c *gin.Context) {
	userIdStr, _ := c.Get("userId")
	userID, _ := primitive.ObjectIDFromHex(userIdStr.(string))

	var input models.ChangePasswordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request body"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	user, err := h.Repo.GetByID(ctx, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("User not found"))
		return
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.CurrentPassword)); err != nil {
		c.JSON(http.StatusUnauthorized, utils.ErrorResponse("Current password is incorrect"))
		return
	}

	// Update password
	if err := h.Repo.ChangePassword(ctx, userID, input.NewPassword); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to update password"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Password updated successfully", nil))
}
