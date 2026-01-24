package handlers

import (
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/developia-II/ecommerce-backend/internal/models"
	"github.com/developia-II/ecommerce-backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type OnboardingHandler struct {
	DB *mongo.Database
}

func NewOnboardingHandler(db *mongo.Database) *OnboardingHandler {
	return &OnboardingHandler{DB: db}
}

var onboardingValidator = validator.New()

func (h *OnboardingHandler) ClientUpdateInterest(c *gin.Context) {
	// Get userId from context (set by AuthMiddleware)
	userId, _ := c.Get("userId")
	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second*10)
	defer cancel()

	var user models.User
	collection := h.DB.Collection("users")
	objectId, err := primitive.ObjectIDFromHex(userId.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Invalid or missing token"))
		return
	}
	filter := bson.M{"_id": objectId}
	if err := collection.FindOne(ctx, filter).Decode(&user); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("No user found"))
		return
	}

	var userInterest models.UserInterests
	if err := c.ShouldBindJSON(&userInterest); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid JSON format"))
		return
	}
	if err := onboardingValidator.Struct(userInterest); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Validation failed: "+err.Error()))
		return
	}

	// Initialize interests object if it's null, then set the fields
	update := bson.M{
		"$set": bson.M{
			"interests": bson.M{
				"categories": userInterest.Categories,
				"isSet":      true,
			},
			"updatedAt": time.Now(),
		},
	}
	result, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to update user interest: "+err.Error()))
		return
	}
	if result.MatchedCount == 0 {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("User not found"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Interests updated successfully", gin.H{
		"categories":    userInterest.Categories,
		"isInterestSet": true,
	}))
}

func (h *OnboardingHandler) ClientUpdatePreference(c *gin.Context) {
	// Get userId from context (set by AuthMiddleware)
	userIdStr, _ := c.Get("userId")
	objectId, err := primitive.ObjectIDFromHex(userIdStr.(string))
	if err != nil {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("Invalid token"))
		return
	}

	var userPref models.UserPreferences

	if err := c.ShouldBindJSON(&userPref); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Validation failed: "+err.Error()))
		return
	}
	if err := onboardingValidator.Struct(&userPref); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Validation failed: "+err.Error()))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	collection := h.DB.Collection("users")

	var user models.User
	if err := collection.FindOne(ctx, bson.M{"_id": objectId}).Decode(&user); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("No user found"))
		return
	}
	filter := bson.M{"_id": objectId}

	// Initialize preferences object if it's null, then set the fields
	update := bson.M{
		"$set": bson.M{
			"preferences": bson.M{
				"budgetRange":       userPref.BudgetRange,
				"shoppingFrequency": userPref.ShoppingFrequency,
				"specialPrefs":      userPref.SpecialPrefs,
			},
			"updatedAt": time.Now(),
		},
	}
	result, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to update user: "+err.Error()))
		return
	}
	if result.MatchedCount == 0 {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("User not found"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("User preference updated successfully", gin.H{
		"budgetRange":       userPref.BudgetRange,
		"shoppingFrequency": userPref.ShoppingFrequency,
		"specialPrefs":      userPref.SpecialPrefs,
	}))

}

func (h *OnboardingHandler) CompleteOnboardingFlow(c *gin.Context) {

	// Get userId from context (set by AuthMiddleware)
	userIdStr, _ := c.Get("userId")
	objectId, err := primitive.ObjectIDFromHex(userIdStr.(string))

	location := c.PostForm("location")
	bio := c.PostForm("bio")
	file, err := c.FormFile("profile_picture")
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Profile picture is required"))
		return
	}

	allowedTypes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
	}
	if !allowedTypes[file.Header.Get("Content-Type")] {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Only JPEG/PNG images are allowed"))
		return
	}

	cld, err := cloudinary.NewFromParams(
		os.Getenv("CLOUDINARY_CLOUD_NAME"),
		os.Getenv("CLOUDINARY_API_KEY"),
		os.Getenv("CLOUDINARY_API_SECRET"),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to initialize Cloudinary"))
		return
	}

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to process image"))
		return
	}
	defer src.Close()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	uploadResult, err := cld.Upload.Upload(ctx, src, uploader.UploadParams{
		Folder: "users/profiles",
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to upload image"))
		return
	}

	collection := h.DB.Collection("users")
	filter := bson.M{"_id": objectId}

	// Initialize profile object if it's null, then set the fields
	update := bson.M{
		"$set": bson.M{
			"profile": bson.M{
				"location":     location,
				"bio":          bio,
				"profileImage": uploadResult.SecureURL,
			},
			"onboardingCompleted": true,
			"updatedAt":           time.Now(),
		},
	}

	result, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to update profile: "+err.Error()))
		return
	}
	if result.MatchedCount == 0 {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("User not found"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Onboarding completed successfully", gin.H{
		"success":             true,
		"onboardingCompleted": true,
		"profile": gin.H{
			"location":     location,
			"bio":          bio,
			"profileImage": uploadResult.SecureURL,
		},
	}))
}

func (h *OnboardingHandler) UserOnboardingDraft(c *gin.Context) {
	authHeader := c.Request.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("Invalid or missing token"))
		return
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	claims, err := utils.VerifyToken(tokenString)
	if err != nil {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("Failed to verify token"))
		return
	}
	userId := claims.UserID

	objectId, err := primitive.ObjectIDFromHex(userId)
	if err != nil {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("Invalid or missing token"))
		return
	}
	var input models.UserOnboardingDraft
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request body"))
		return
	}
	if err := onboardingValidator.Struct(&input); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Validation failed: "+err.Error()))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	collection := h.DB.Collection("drafts")

	filter := bson.M{"userID": objectId, "role": input.Role}
	update := bson.M{
		"$set": bson.M{
			"step":          input.Step,
			"stepCompleted": input.StepCompleted,
			"stepData":      input.StepData,
			"role":          input.Role,
			"updatedAt":     time.Now(),
		},
		"$setOnInsert": bson.M{
			"userID": objectId,
		},
		"$inc": bson.M{"version": 1},
	}

	opts := options.FindOneAndUpdate().
		SetUpsert(true).
		SetReturnDocument(options.After)

	_, _ = h.DB.Collection("drafts").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "userID", Value: 1}, {Key: "role", Value: 1}},
		Options: options.Index().SetUnique(true),
	})

	var saved models.UserOnboardingDraft
	if err := collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&saved); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to save draft"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Draft saved successfully", gin.H{
		"success": true,
		"data": gin.H{
			"id":            saved.ID,
			"userID":        saved.UserID,
			"role":          saved.Role,
			"step":          saved.Step,
			"stepCompleted": saved.StepCompleted,
			"stepData":      saved.StepData,
			"version":       saved.Version,
			"updatedAt":     saved.UpdatedAt,
		},
	}))
}

func (h *OnboardingHandler) GetOnboardingDraft(c *gin.Context) {
	authHeader := c.Request.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("Invalid or missing token"))
		return
	}
	claims, err := utils.VerifyToken(strings.TrimPrefix(authHeader, "Bearer "))
	if err != nil {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("Invalid or expired token"))
		return
	}
	role := c.Query("role")
	if role == "" {
		role = "customer"
	}

	objectId, err := primitive.ObjectIDFromHex(claims.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid user ID"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	var draft models.UserOnboardingDraft
	err = h.DB.Collection("drafts").FindOne(ctx, bson.M{
		"userID": objectId,
		"role":   role,
	}).Decode(&draft)
	if err == mongo.ErrNoDocuments {
		c.JSON(http.StatusOK, utils.SuccessResponse("No draft found", gin.H{"success": true, "data": nil}))
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to fetch draft"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Draft fetched successfully", gin.H{
		"success": true,
		"data": gin.H{
			"id":            draft.ID,
			"userID":        draft.UserID,
			"role":          draft.Role,
			"step":          draft.Step,
			"stepCompleted": draft.StepCompleted,
			"stepData":      draft.StepData,
			"version":       draft.Version,
			"updatedAt":     draft.UpdatedAt,
		},
	}))
}

func (h *OnboardingHandler) SellerBusinessType(c *gin.Context) {
	auth := c.GetHeader("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("Invalid or missing token"))
		return
	}
	claims, err := utils.VerifyToken(strings.TrimPrefix(auth, "Bearer "))
	if err != nil {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("Invalid or missing token"))
		return
	}

	var input models.SellerBusinessInfo

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid json format"))
		return
	}
	if err := onboardingValidator.Struct(input); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid json payload: "+err.Error()))
		return
	}

	userID, _ := primitive.ObjectIDFromHex(claims.UserID)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Save to drafts (role=vendor)
	filter := bson.M{"userID": userID, "role": "vendor"}
	update := bson.M{
		"$set": bson.M{
			"stepData.businessInfo": input,
			"updatedAt":             time.Now(),
		},
		"$setOnInsert": bson.M{"userID": userID, "role": "vendor", "step": 1},
		"$inc":         bson.M{"version": 1},
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)

	var saved models.UserOnboardingDraft
	if err := h.DB.Collection("drafts").FindOneAndUpdate(ctx, filter, update, opts).Decode(&saved); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to save: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Business details updated", gin.H{
		"success": true,
		"data":    gin.H{"businessInfo": input},
	}))
}

func (h *OnboardingHandler) SellerBusinessCategory(c *gin.Context) {
	auth := c.GetHeader("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("Invalid or missing token"))
		return
	}
	claims, err := utils.VerifyToken(strings.TrimPrefix(auth, "Bearer "))
	if err != nil {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("Invalid or missing token"))
		return
	}

	type categoryInput struct {
		Categories []string `json:"categories" validate:"required,min=1,max=5,dive,required"`
	}
	var input categoryInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid json format"))
		return
	}
	if err := onboardingValidator.Struct(input); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid json payload: "+err.Error()))
		return
	}

	userID, _ := primitive.ObjectIDFromHex(claims.UserID)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	filter := bson.M{"userID": userID, "role": "vendor"}
	update := bson.M{
		"$set": bson.M{
			"stepData.categories": input.Categories,
			"updatedAt":           time.Now(),
		},
		"$setOnInsert": bson.M{"userID": userID, "role": "vendor", "step": 2},
		"$inc":         bson.M{"version": 1},
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)

	var saved models.UserOnboardingDraft
	if err := h.DB.Collection("drafts").FindOneAndUpdate(ctx, filter, update, opts).Decode(&saved); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to save: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Business categories updated", gin.H{
		"success": true,
		"data":    gin.H{"categories": input.Categories},
	}))
}

func (h *OnboardingHandler) SellerBusinessInfo(c *gin.Context) {
	auth := c.GetHeader("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("Invalid or missing token"))
		return
	}
	claims, err := utils.VerifyToken(strings.TrimPrefix(auth, "Bearer "))
	if err != nil {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("Invalid or missing token"))
		return
	}
	var input models.BusinessDetails
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid json payload"))
		fmt.Println("Error", err)
		return
	}
	if err := onboardingValidator.Struct(&input); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid json payload"))
		fmt.Println("Error", err)
		return
	}
	userID, _ := primitive.ObjectIDFromHex(claims.UserID)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	filter := bson.M{
		"userID": userID,
		"role":   "vendor",
	}
	update := bson.M{
		"$set": bson.M{
			"stepData.businessDetails": bson.M{
				"businessName": input.BusinessName,
				"description":  input.Description,
				"location":     input.Location,
				"url":          input.Url,
			},
			"updatedAt": time.Now(),
		},
		"$setOnInsert": bson.M{"userID": userID, "role": "vendor", "step": 3},
		"$inc":         bson.M{"version": 1},
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	var saved models.UserOnboardingDraft
	if err := h.DB.Collection("drafts").FindOneAndUpdate(ctx, filter, update, opts).Decode(&saved); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to save: "+err.Error()))
		return
	}
	c.JSON(http.StatusOK, utils.SuccessResponse("Business details updated", gin.H{
		"success": true,
		"data": gin.H{
			"businessName": input.BusinessName,
			"description":  input.Description,
			"location":     input.Location,
			"url":          input.Url,
		},
	}))
}

func (h *OnboardingHandler) StoreDetails(c *gin.Context) {
	authHeader := c.Request.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("Invalid or missing token"))
		return
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := utils.VerifyToken(tokenString)
	if err != nil {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("Invalid or expired token"))
		return
	}
	userID, _ := primitive.ObjectIDFromHex(claims.UserID)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	storeName := c.PostForm("storeName")
	storeDescription := c.PostForm("storeDescription")
	primaryColor := c.PostForm("primaryColor")
	accentColor := c.PostForm("accentColor")
	file, err := c.FormFile("storeLogo")
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid file"))
		return
	}

	allowedTypes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
	}
	if !allowedTypes[file.Header.Get("Content-Type")] {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Only JPEG/PNG images are allowed"))
		return
	}
	cld, err := cloudinary.NewFromParams(
		os.Getenv("CLOUDINARY_CLOUD_NAME"),
		os.Getenv("CLOUDINARY_API_KEY"),
		os.Getenv("CLOUDINARY_API_SECRET"),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to initialize Cloudinary"))
		return
	}

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to process image"))
		return
	}
	defer src.Close()

	uploadResult, err := cld.Upload.Upload(ctx, src, uploader.UploadParams{
		Folder: "stores/logo",
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to upload image"))
		return
	}

	collection := h.DB.Collection("drafts")
	filter := bson.M{
		"userID": userID,
		"role":   "vendor",
	}
	update := bson.M{
		"$set": bson.M{
			"stepData.storeDetails": bson.M{
				"storeLogo":        uploadResult.SecureURL,
				"storeName":        storeName,
				"storeDescription": storeDescription,
				"primaryColor":     primaryColor,
				"accentColor":      accentColor,
			},
			"updatedAt": time.Now(),
		},
		"$setOnInsert": bson.M{"userID": userID, "role": "vendor", "step": 4},
		"$inc":         bson.M{"version": 1},
	}

	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	var saved models.UserOnboardingDraft
	if err := collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&saved); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to update store details"))
		return
	}
	c.JSON(http.StatusOK, utils.SuccessResponse("Store details updated", gin.H{
		"success": true,
		"data": gin.H{
			"storeLogo":        uploadResult.SecureURL,
			"storeName":        storeName,
			"storeDescription": storeDescription,
			"primaryColor":     primaryColor,
			"accentColor":      accentColor,
		},
	}))
}
func (h *OnboardingHandler) CalculateTier1RiskScore(user *models.User, application *models.SellerApplication) models.RiskScore {
	score := 0
	var flags []string

	// 1. Email quality checks (25 points max)
	if utils.IsDisposableEmail(user.Email) {
		score += 25
		flags = append(flags, "Disposable email domain detected")
	}

	// 2. Account age checks (35 points max)
	accountAge := time.Since(user.CreatedAt)
	if accountAge < 1*time.Hour {
		score += 30 // Ultra-new account - almost certainly needs review
		flags = append(flags, "Account created < 1 hour ago")
	} else if accountAge < 24*time.Hour {
		score += 15
		flags = append(flags, "Account created < 24 hours ago")
	}

	// 3. Document Quality & Validation (30 points max)
	if application.IDDocument != nil {
		// Suspiciously small files often indicate low-quality/placeholder images
		if application.IDDocument.FileSize < 150*1024 { // Increased to 150KB
			score += 15
			flags = append(flags, "Low resolution or suspicious document size")
		}

		ext := strings.ToLower(application.IDDocument.ContentType)
		if ext != "image/jpeg" && ext != "image/png" && ext != "application/pdf" {
			score += 20 // Higher penalty for weird formats
			flags = append(flags, "Non-standard document format")
		}
	}

	// 4. Contextual Consistency (20 points max)
	// Check if the business location matches the user's previously provided profile location
	if user.Profile != nil && application.BusinessDetails != nil {
		if !strings.EqualFold(user.Profile.Location, application.BusinessDetails.Location) {
			score += 10
			flags = append(flags, "Location mismatch with profile")
		}
	}

	// 5. Store Metadata Checks (15 points max)
	if len(application.StoreName) < 4 {
		score += 10
		flags = append(flags, "Unusually short store name")
	}

	// Check for "Bot-like" behavior: If they submitted instantly after creating account
	if application.AppliedAt.Sub(user.CreatedAt) < 10*time.Minute {
		score += 20
		flags = append(flags, "Rapid application submission (bot risk)")
	}

	return models.RiskScore{
		Total:     score,
		Threshold: 35, // Adjusted threshold for auto-approval
		Flags:     flags,
	}
}
func (h *OnboardingHandler) MakeApprovalDecision(score models.RiskScore) models.ApprovalDecision {
	// 0-35: Trusted / Low Risk (Safe for Auto-Approve)
	if score.Total < 35 {
		return models.ApprovalDecision{
			Action: "AUTO_APPROVE",
			Reason: "Low risk profile - automated approval granted",
			Score:  score.Total,
		}
	}

	// 35-65: Suspicious / Medium Risk (Requires Human Review)
	if score.Total >= 35 && score.Total < 65 {
		return models.ApprovalDecision{
			Action: "MANUAL_REVIEW",
			Reason: "Elevated risk flags - queued for manual review",
			Score:  score.Total,
		}
	}

	// 65+: High Risk (Auto-Reject to protect platform)
	return models.ApprovalDecision{
		Action: "REJECT",
		Reason: "High risk profile - automated rejection based on security policy",
		Score:  score.Total,
	}
}
func (h *OnboardingHandler) processDocumentUpload(form *multipart.Form, fieldName string) *models.VerificationDocument {
	headers, ok := form.File[fieldName]
	if !ok || len(headers) == 0 {
		return nil
	}
	header := headers[0]

	// Validate file size (5MB max)
	if header.Size > 5*1024*1024 {
		return nil
	}

	// Validate file type
	allowedTypes := map[string]bool{
		"image/jpeg":      true,
		"image/png":       true,
		"application/pdf": true,
	}

	if !allowedTypes[header.Header.Get("Content-Type")] {
		return nil
	}

	file, err := header.Open()
	if err != nil {
		return nil
	}
	defer file.Close()

	// Upload to Cloudinary
	uploadResult, err := h.uploadToCloudinary(file, header)
	if err != nil {
		return nil
	}

	return &models.VerificationDocument{
		FileName:    header.Filename,
		FileURL:     uploadResult.SecureURL,
		FileSize:    header.Size,
		ContentType: header.Header.Get("Content-Type"),
		UploadedAt:  time.Now(),
	}
}

func (h *OnboardingHandler) uploadToCloudinary(file multipart.File, header *multipart.FileHeader) (*uploader.UploadResult, error) {
	// Use your existing Cloudinary setup from store details upload
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create Cloudinary instance using the same pattern as StoreDetails
	cld, err := cloudinary.NewFromParams(
		os.Getenv("CLOUDINARY_CLOUD_NAME"),
		os.Getenv("CLOUDINARY_API_KEY"),
		os.Getenv("CLOUDINARY_API_SECRET"),
	)
	if err != nil {
		return nil, err
	}

	uploadResult, err := cld.Upload.Upload(ctx, file, uploader.UploadParams{
		Folder:       "seller-verification",
		ResourceType: "auto",
		Format:       "auto",
	})

	return uploadResult, err
}
func (h *OnboardingHandler) SellerVerification(c *gin.Context) {
	auth := c.GetHeader("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("Invalid or missing token"))
		return
	}
	claims, err := utils.VerifyToken(strings.TrimPrefix(auth, "Bearer "))
	if err != nil {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("Invalid or missing token"))
		return
	}

	// Parse form data (documents)
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid form data"))
		return
	}

	userID, _ := primitive.ObjectIDFromHex(claims.UserID)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 1. Check if user already has a pending or approved application
	var existingApp models.SellerApplication
	err = h.DB.Collection("sellerApplications").FindOne(ctx, bson.M{
		"userID": userID,
		"status": bson.M{"$in": []string{"pending", "approved", "under_review"}},
	}).Decode(&existingApp)
	if err == nil {
		c.JSON(http.StatusConflict, utils.ErrorResponse("You already have an active application or verified account"))
		return
	}

	// 2. Get user data for risk scoring
	var user models.User
	if err := h.DB.Collection("users").FindOne(ctx, bson.M{"_id": userID}).Decode(&user); err != nil {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("User not found"))
		return
	}

	// 3. Get draft data to populate application
	var draft models.UserOnboardingDraft
	err = h.DB.Collection("drafts").FindOne(ctx, bson.M{"userID": userID, "role": "vendor"}).Decode(&draft)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Onboarding draft not found. Please complete previous steps."))
		return
	}

	// 4. Process uploaded documents
	idDocument := h.processDocumentUpload(form, "idDocument")
	if idDocument == nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("ID document required"))
		return
	}
	selfieDoc := h.processDocumentUpload(form, "selfieVerification")

	// 5. Build SellerApplication from draft and uploads
	application := &models.SellerApplication{
		ID:                 primitive.NewObjectID(),
		UserID:             userID,
		RequestedTier:      "individual", // Default
		IDDocument:         idDocument,
		SelfieVerification: selfieDoc,
		Status:             "pending",
		AppliedAt:          time.Now(),
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	// Extract data from draft.StepData with safety
	if busInfo, ok := draft.StepData["businessInfo"].(map[string]interface{}); ok {
		application.BusinessTypeInfo = &models.SellerBusinessInfo{}
		if val, ok := busInfo["requestedTier"].(string); ok {
			application.RequestedTier = val
			application.BusinessTypeInfo.RequestedTier = val
		}
		if val, ok := busInfo["type"].(string); ok {
			application.BusinessTypeInfo.BusinessType = val
		}
		if val, ok := busInfo["size"].(string); ok {
			application.BusinessTypeInfo.BusinessSize = val
		}
		if val, ok := busInfo["experience"].(string); ok {
			application.BusinessTypeInfo.BusinessExperience = val
		}

		if application.BusinessTypeInfo.BusinessType != "" && application.BusinessTypeInfo.BusinessType != "unregistered" {
			application.IsRegistered = true
		}
	}

	if categories, ok := draft.StepData["categories"].([]interface{}); ok {
		for _, cat := range categories {
			if s, ok := cat.(string); ok {
				application.Categories = append(application.Categories, s)
			}
		}
	}

	if busDetails, ok := draft.StepData["businessDetails"].(map[string]interface{}); ok {
		application.BusinessDetails = &models.BusinessDetails{}
		if val, ok := busDetails["businessName"].(string); ok {
			application.BusinessDetails.BusinessName = val
		}
		if val, ok := busDetails["description"].(string); ok {
			application.BusinessDetails.Description = val
		}
		if val, ok := busDetails["location"].(string); ok {
			application.BusinessDetails.Location = val
		}
		if val, ok := busDetails["url"].(string); ok {
			application.BusinessDetails.Url = val
		}

		application.StoreName = application.BusinessDetails.BusinessName
	}

	if storeDetails, ok := draft.StepData["storeDetails"].(map[string]interface{}); ok {
		application.StoreDetails = &models.StoreDetails{}
		if val, ok := storeDetails["storeLogo"].(string); ok {
			application.StoreDetails.StoreLogo = val
		}
		if val, ok := storeDetails["storeName"].(string); ok {
			application.StoreDetails.StoreName = val
		}
		if val, ok := storeDetails["storeDescription"].(string); ok {
			application.StoreDetails.StoreDescription = val
		}
		if val, ok := storeDetails["primaryColor"].(string); ok {
			application.StoreDetails.PrimaryColor = val
		}
		if val, ok := storeDetails["accentColor"].(string); ok {
			application.StoreDetails.AccentColor = val
		}

		if application.StoreDetails.StoreName != "" {
			application.StoreName = application.StoreDetails.StoreName
		}
		if application.StoreDetails.StoreDescription != "" {
			application.StoreDescription = application.StoreDetails.StoreDescription
		}
	}

	// 6. Calculate risk score
	riskScore := h.CalculateTier1RiskScore(&user, application)

	// 7. Make approval decision
	decision := h.MakeApprovalDecision(riskScore)

	// 8. Update application status based on decision
	application.RiskScore = riskScore.Total
	application.RiskFlags = riskScore.Flags

	// Only auto-approve Tier 1 (individual)
	if application.RequestedTier == "individual" && decision.Action == "AUTO_APPROVE" {
		application.Status = "approved"
		application.ApprovedTier = "individual"
	} else if decision.Action == "REJECT" {
		application.Status = "rejected"
		application.RejectionReason = decision.Reason
	} else {
		application.Status = "pending" // Requires manual review (Tier 2/3 or Medium/High risk)
	}

	// 9. Save application
	_, err = h.DB.Collection("sellerApplications").InsertOne(ctx, application)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to save application"))
		return
	}

	// 10. If approved, create vendor account and update user role
	if application.Status == "approved" {
		vendorAccount := &models.VendorAccount{
			ID:              primitive.NewObjectID(),
			UserID:          userID,
			ApplicationID:   application.ID,
			Tier:            "individual",
			MaxProducts:     50,
			MaxMonthlySales: 5000,
			TransactionFee:  5.0,
			PayoutHoldDays:  7,
			Status:          "active",
			ActivatedAt:     time.Now(),
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}

		_, err = h.DB.Collection("vendorAccounts").InsertOne(ctx, vendorAccount)
		if err == nil {
			// Update user role and vendor status
			h.DB.Collection("users").UpdateOne(ctx, bson.M{"_id": userID}, bson.M{
				"$set": bson.M{
					"role":         "vendor",
					"vendorStatus": "approved",
					"updatedAt":    time.Now(),
				},
			})
		}
	} else {
		// Update user vendor status to pending (or rejected)
		h.DB.Collection("users").UpdateOne(ctx, bson.M{"_id": userID}, bson.M{
			"$set": bson.M{
				"vendorStatus": application.Status,
				"updatedAt":    time.Now(),
			},
		})
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Verification processed", gin.H{
		"applicationID": application.ID,
		"status":        application.Status,
		"decision":      decision.Action,
		"riskScore":     riskScore.Total,
		"flags":         riskScore.Flags,
	}))
}
