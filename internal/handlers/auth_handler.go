package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/developia-II/ecommerce-backend/internal/models"
	"github.com/developia-II/ecommerce-backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	DB *mongo.Database
}

func NewAuthHandler(db *mongo.Database) *AuthHandler {
	return &AuthHandler{DB: db}
}

var validate = validator.New()

func (h *AuthHandler) CreateUser(c *gin.Context) {
	var user models.RegisterInput

	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request body"))
		return
	}

	if err := validate.Struct(user); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse(err.Error()))
		return
	}
	collection := h.DB.Collection("users")

	var existingUser models.User

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := collection.FindOne(ctx, bson.M{"email": user.Email}).Decode(&existingUser); err == nil {
		c.JSON(http.StatusConflict, utils.ErrorResponse("Email already exists"))
		return
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to hash password"))
		return
	}
	newUser := models.User{
		ID:         primitive.NewObjectID(),
		Email:      user.Email,
		Name:       user.Name,
		Phone:      user.Phone,
		Address:    user.Address,
		Password:   string(hashedPassword),
		Role:       "buyer",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		IsVerified: false,
	}

	if _, err := collection.InsertOne(ctx, newUser); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to register user"))
		return
	}
	verificationLink := fmt.Sprintf("http://localhost:3000/verify?token=%s", newUser.ID.Hex())
	emailBody := fmt.Sprintf(`
    <html>
    <body style="font-family: Arial, sans-serif;">
        <h2>Welcome to Vendora, %s!</h2>
        <p>Thank you for registering. Please verify your email by clicking the button below:</p>
        <a href="%s" style="background-color: #4CAF50; color: white; padding: 10px 20px; text-decoration: none; border-radius: 5px;">Verify Email</a>
        <p>If you didn't create this account, please ignore this email.</p>
        <p>Best regards,<br>The Vendora Team</p>
    </body>
    </html>
`, newUser.Name, verificationLink)

	go func() {
		if err := utils.SendEmail(user.Email, "Verify Your Vendora Account", emailBody); err != nil {
			logrus.WithError(err).WithField("email", user.Email).Error("Failed to send verification email")
		} else {
			logrus.WithField("email", user.Email).Info("Verification email sent successfully")
		}
	}()

	accessToken, err := utils.GenerateToken(newUser.ID.Hex(), newUser.Role, 24*time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("failed to generate accessToken"))
		return
	}

	refreshToken, err := utils.GenerateRefreshToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("failed to generate refreshToken"))
		return
	}

	refreshTokenExpiry := time.Now().Add(7 * 24 * time.Hour) // 7 days
	update := bson.M{
		"$set": bson.M{
			"refreshToken":       refreshToken,
			"refreshTokenExpiry": refreshTokenExpiry,
		},
	}

	if _, err := collection.UpdateOne(ctx, bson.M{"_id": newUser.ID}, update); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("failed to save refresh token"))
		return
	}

	response := gin.H{
		"message": "User created successfully",
		"user": gin.H{
			"id":    newUser.ID.Hex(),
			"role":  newUser.Role,
			"name":  newUser.Name,
			"email": newUser.Email,
		},
		"accessToken":  accessToken,
		"refreshToken": refreshToken,
		"isVerified":   newUser.IsVerified,
	}
	c.JSON(http.StatusCreated, utils.SuccessResponse("User Created Successfully", response))

}

func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	token := c.Param("token")

	userID, err := primitive.ObjectIDFromHex(token)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid token"))
		return
	}

	collection := h.DB.Collection("users")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	update := bson.M{"$set": bson.M{"isVerified": true}}
	var updatedUser models.User
	if err := collection.FindOneAndUpdate(ctx, bson.M{"_id": userID}, update).Decode(&updatedUser); err != nil {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("User not found"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Email verified successfully", gin.H{
		"success": true,
		"data": gin.H{
			"id":                  updatedUser.ID.Hex(),
			"role":                updatedUser.Role,
			"name":                updatedUser.Name,
			"email":               updatedUser.Email,
			"isVerified":          updatedUser.IsVerified,
			"onboardingCompleted": updatedUser.OnboardingCompleted,
		},
	}))
}

func (h *AuthHandler) ResendVerification(c *gin.Context) {
	token := c.Param("token")

	userID, err := primitive.ObjectIDFromHex(token)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid token"))
		return
	}

	collection := h.DB.Collection("users")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	var user models.User
	err = collection.FindOne(ctx, bson.M{"_id": userID}).Decode(&user)
	if err != nil {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("User not found"))
		return
	}

	// Check if user is already verified
	if user.IsVerified {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("User is already verified"))
		return
	}

	// Generate new verification link (using the same user ID as token)
	verificationLink := fmt.Sprintf("http://localhost:3000/verify?token=%s", user.ID.Hex())
	emailBody := fmt.Sprintf(`
    <html>
    <body style="font-family: Arial, sans-serif;">
        <h2>Welcome to Vendora, %s!</h2>
        <p>Here is your verification link again. Please verify your email by clicking the button below:</p>
        <a href="%s" style="background-color: #4CAF50; color: white; padding: 10px 20px; text-decoration: none; border-radius: 5px;">Verify Email</a>
        <p>If you didn't create this account, please ignore this email.</p>
        <p>Best regards,<br>The Vendora Team</p>
    </body>
    </html>
`, user.Name, verificationLink)

	go func() {
		if err := utils.SendEmail(user.Email, "Verify Your Vendora Account", emailBody); err != nil {
			logrus.WithError(err).WithField("email", user.Email).Error("Failed to send verification email")
		} else {
			logrus.WithField("email", user.Email).Info("Verification email sent successfully")
		}
	}()

	c.JSON(http.StatusOK, utils.SuccessResponse("Verification email sent successfully", nil))
}

func (h *AuthHandler) LoginUser(c *gin.Context) {
	var cred struct {
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&cred); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request body"))
		return
	}
	if err := validate.Struct(cred); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse(err.Error()))
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	var user models.User
	collection := h.DB.Collection("users")
	filter := bson.M{"email": cred.Email}
	if err := collection.FindOne(ctx, filter).Decode(&user); err != nil {
		c.JSON(http.StatusUnauthorized, utils.ErrorResponse("Invalid email or password"))
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(cred.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, utils.ErrorResponse("Invalid email or password")) // Same message
		return
	}
	if !user.IsVerified {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("Please verify your account"))
		return
	}
	token, err := utils.GenerateToken(user.ID.Hex(), user.Role, 24*time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to generate token"))
		return
	}

	refreshToken, err := utils.GenerateRefreshToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to generate refresh token"))
		return
	}

	refreshTokenExpiry := time.Now().Add(7 * 24 * time.Hour) // 7 days
	update := bson.M{
		"$set": bson.M{
			"refreshToken":       refreshToken,
			"refreshTokenExpiry": refreshTokenExpiry,
		},
	}

	if _, err := collection.UpdateOne(ctx, bson.M{"_id": user.ID}, update); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to save refresh token"))
		return
	}
	res := gin.H{
		"success": true,
		"user": gin.H{
			"name":    user.Name,
			"email":   user.Email,
			"address": user.Address,
			"role":    user.Role,
		},
		"accessToken":  token,
		"refreshToken": refreshToken,
	}
	c.JSON(http.StatusAccepted, utils.SuccessResponse("Login Successfull", res))
}

func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var input struct {
		Email string `json:"email" validate:"required,email"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid JSON"))
		logrus.WithError(err).Info(err.Error())
		return
	}
	if err := validate.Struct(input); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse(err.Error()))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	var user models.User
	collection := h.DB.Collection("users")
	filter := bson.M{"email": input.Email}
	if err := collection.FindOne(ctx, filter).Decode(&user); err != nil {
		c.JSON(http.StatusOK, utils.SuccessResponse("If the email exists, a reset link has been sent", nil))
		return
	}
	resetToken, err := utils.GenerateSecureToken(32)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to generate reset token"))
		return
	}
	resetTokenExpiry := time.Now().Add(1 * time.Hour)
	update := bson.M{
		"$set": bson.M{
			"resetToken":       resetToken,
			"resetTokenExpiry": resetTokenExpiry,
		},
	}

	if _, err := collection.UpdateOne(ctx, bson.M{"_id": user.ID}, update); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to save reset token"))
		return
	}
	resetLink := fmt.Sprintf("http://localhost:3000/reset-password?token=%s", resetToken)
	emailBody := fmt.Sprintf(`
        <html>
        <body style="font-family: Arial, sans-serif;">
            <h2>Password Reset Request</h2>
            <p>Hi %s,</p>
            <p>You requested to reset your password. Click the button below to reset it:</p>
            <a href="%s" style="background-color: #4CAF50; color: white; padding: 10px 20px; text-decoration: none; border-radius: 5px;">Reset Password</a>
            <p>This link will expire in 1 hour.</p>
            <p>If you didn't request this, please ignore this email.</p>
            <p>Best regards,<br>The Vendora Team</p>
        </body>
        </html>
    `, user.Name, resetLink)

	go func() {
		if err := utils.SendEmail(user.Email, "Reset Your Vendora Password", emailBody); err != nil {
			logrus.WithError(err).WithField("email", user.Email).Error("Failed to send reset email")
		} else {
			logrus.WithField("email", user.Email).Info("Reset email sent successfully")
		}
	}()

	c.JSON(http.StatusOK, utils.SuccessResponse("If the email exists, a reset link has been sent", nil))

}

func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var input struct {
		Token       string `json:"token" validate:"required"`
		NewPassword string `json:"newPassword" validate:"required,min=8"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request body"))
		return
	}

	if err := validate.Struct(input); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse(err.Error()))
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if input.Token == "" {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Missing reset token"))
		return
	}

	collection := h.DB.Collection("users")
	filter := bson.M{"resetToken": input.Token}
	var user models.User
	if err := collection.FindOne(ctx, filter).Decode(&user); err != nil {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("Invalid Reset Token"))
		return
	}
	if time.Now().After(user.ResetTokenExpiry) {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("Reset token has expired"))
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to hash password"))
		return
	}

	update := bson.M{
		"$set": bson.M{
			"password": string(hashedPassword),
		},
		"$unset": bson.M{
			"resetToken":       "",
			"resetTokenExpiry": "",
		},
	}

	res, err := collection.UpdateOne(ctx, bson.M{"_id": user.ID}, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to update password"))
		return
	}
	if res.MatchedCount == 0 {
		c.JSON(http.StatusOK, utils.SuccessResponse("No user found", gin.H{}))
		return
	}

	response := gin.H{
		"success": true,
		"user": gin.H{
			"name":  user.Name,
			"role":  user.Role,
			"email": user.Email,
		},
	}
	c.JSON(http.StatusOK, utils.SuccessResponse("Password updated successfully", response))

}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var input struct {
		RefreshToken string `json:"refreshToken" validate:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request body"))
		return
	}

	if err := validate.Struct(input); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse(err.Error()))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	collection := h.DB.Collection("users")
	filter := bson.M{"refreshToken": input.RefreshToken}

	var user models.User
	if err := collection.FindOne(ctx, filter).Decode(&user); err != nil {
		c.JSON(http.StatusUnauthorized, utils.ErrorResponse("Invalid refresh token"))
		return
	}

	if time.Now().After(user.RefreshTokenExpiry) {
		c.JSON(http.StatusUnauthorized, utils.ErrorResponse("Refresh token expired"))
		return
	}

	accessToken, err := utils.GenerateToken(user.ID.Hex(), user.Role, 24*time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to generate access token"))
		return
	}

	response := gin.H{
		"accessToken": accessToken,
		"user": gin.H{
			"id":    user.ID.Hex(),
			"role":  user.Role,
			"name":  user.Name,
			"email": user.Email,
		},
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Token refreshed successfully", response))
}
