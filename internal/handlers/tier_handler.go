package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/developia-II/ecommerce-backend/internal/adapters/repository"
	"github.com/developia-II/ecommerce-backend/internal/models"
	"github.com/developia-II/ecommerce-backend/internal/services"
	"github.com/developia-II/ecommerce-backend/utils"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type TierHandler struct {
	Repo      repository.TierRepository
	UserRepo  repository.UserRepository
	AIService *services.VerificationService
}

func NewTierHandler(db *mongo.Database) *TierHandler {
	aiService, _ := services.NewVerificationService()
	return &TierHandler{
		Repo:      repository.NewTierRepository(db),
		UserRepo:  repository.NewUserRepository(db),
		AIService: aiService,
	}
}

func (h *TierHandler) RequestUpgrade(c *gin.Context) {
	userIdStr, _ := c.Get("userId")
	vendorID, _ := primitive.ObjectIDFromHex(userIdStr.(string))

	var input struct {
		RequestedTier string                        `json:"requestedTier" binding:"required"`
		Documents     []models.VerificationDocument `json:"documents"`
		BusinessInfo  *models.BusinessDetails       `json:"businessInfo"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request body"))
		return
	}

	// Increase timeout for AI processing
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	// 1. Check for existing pending request
	existing, err := h.Repo.GetPendingUpgradeRequest(ctx, vendorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to verify existing requests"))
		return
	}
	if existing != nil {
		c.JSON(http.StatusConflict, utils.ErrorResponse("You already have an upgrade request pending review"))
		return
	}

	// 2. Get current tier
	user, err := h.UserRepo.GetByID(ctx, vendorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to fetch vendor account"))
		return
	}

	currentTier := "individual"
	if user.VendorAccount != nil {
		currentTier = user.VendorAccount.Tier
	}

	// Enforce sequential tier upgrades
	if currentTier == "individual" && input.RequestedTier != "verified" {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("Individual sellers can only upgrade to Verified tier"))
		return
	}
	if currentTier == "verified" && input.RequestedTier != "business" {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("Verified sellers can only upgrade to Business tier"))
		return
	}
	if currentTier == "business" {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("You are already on the highest tier"))
		return
	}

	// 3. Handle AI Verification for Tier 2 (Verified)
	status := models.UpgradeStatusPending
	riskScore := 0
	reviewNotes := ""
	adminNotes := ""

	if input.RequestedTier == "verified" && h.AIService != nil {
		var idURL, selfieURL string
		for _, doc := range input.Documents {
			if doc.DocumentType == "government_id" {
				idURL = doc.FileURL
			} else if doc.DocumentType == "selfie" {
				selfieURL = doc.FileURL
			}
		}

		if idURL != "" {
			aiResult, err := h.AIService.AnalyzeIdentity(ctx, idURL, selfieURL, user.Name)
			if err == nil {
				riskScore = 100 - aiResult.Confidence
				reviewNotes = fmt.Sprintf("AI Extraction: %s. Match: %v. Confidence: %d%%. Reason: %s",
					aiResult.ExtractedName, aiResult.IsMatch, aiResult.Confidence, aiResult.RejectionReason)

				if aiResult.IsMatch && aiResult.Confidence > 80 {
					// Auto-approve
					status = models.UpgradeStatusApproved
					h.UserRepo.UpdateTier(ctx, vendorID, "verified")
				} else if !aiResult.IsMatch || aiResult.Confidence < 40 {
					// Hard AI rejection — save as rejected so it appears in history + counts retries
					status = models.UpgradeStatusRejected
					adminNotes = "AI hard rejection: " + aiResult.RejectionReason

					// Increment retries and possibly suspend
					if user.VendorAccount != nil {
						newRetries := user.VendorAccount.VerificationRetries + 1
						if newRetries >= 3 {
							suspendUntil := time.Now().Add(7 * 24 * time.Hour)
							h.UserRepo.UpdateVendorSuspension(ctx, vendorID, newRetries, &suspendUntil)
						} else {
							h.UserRepo.UpdateVendorSuspension(ctx, vendorID, newRetries, nil)
						}
					}
				}
				// Otherwise remains pending (confidence 40-80) for manual review
			}
		}
	}

	// 4. Create request record (ALWAYS saved, including AI rejections)
	req := models.TierUpgradeRequest{
		ID:            primitive.NewObjectID(),
		VendorID:      vendorID,
		CurrentTier:   currentTier,
		RequestedTier: input.RequestedTier,
		Documents:     input.Documents,
		BusinessInfo:  input.BusinessInfo,
		Status:        status,
		RiskScore:     riskScore,
		ReviewNotes:   reviewNotes,
		AdminNotes:    adminNotes,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := h.Repo.CreateUpgradeRequest(ctx, req); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to submit upgrade request"))
		return
	}

	message := "Upgrade request submitted for review."
	if status == models.UpgradeStatusApproved {
		message = "AI Verification successful! Your account has been upgraded to Verified Tier."
	} else if status == models.UpgradeStatusRejected {
		message = "AI Verification failed. Please review the notes and try again."
	}

	c.JSON(http.StatusCreated, utils.SuccessResponse(message, gin.H{
		"requestId": req.ID.Hex(),
		"status":    req.Status,
		"notes":     adminNotes,
	}))
}

func (h *TierHandler) GetUpgradeStatus(c *gin.Context) {
	userIdStr, _ := c.Get("userId")
	vendorID, _ := primitive.ObjectIDFromHex(userIdStr.(string))

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	req, err := h.Repo.GetLatestUpgradeRequest(ctx, vendorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to fetch request status"))
		return
	}

	if req == nil {
		c.JSON(http.StatusOK, utils.SuccessResponse("No pending requests", nil))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Pending request found", gin.H{"request": req}))
}

func (h *TierHandler) GetUpgradeHistory(c *gin.Context) {
	userIdStr, _ := c.Get("userId")
	vendorID, _ := primitive.ObjectIDFromHex(userIdStr.(string))

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	history, err := h.Repo.GetUpgradeHistory(ctx, vendorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to fetch upgrade history"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Upgrade history fetched successfully", gin.H{"history": history}))
}

func (h *TierHandler) GetEligibility(c *gin.Context) {
	userIdStr, _ := c.Get("userId")
	vendorID, _ := primitive.ObjectIDFromHex(userIdStr.(string))

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	user, err := h.UserRepo.GetByID(ctx, vendorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to fetch vendor account"))
		return
	}

	if user.VendorAccount == nil {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("Vendor account not initialized"))
		return
	}

	acc := user.VendorAccount
	currentTier := acc.Tier

	eligibility := gin.H{
		"currentTier":    currentTier,
		"canUpgrade":     false,
		"nextTier":       "",
		"progress":       0,
		"requirements":   []gin.H{},
		"retries":        acc.VerificationRetries,
		"isSuspended":    acc.Status == "suspended",
		"suspendedUntil": acc.SuspendedUntil,
		"appealStatus":   acc.AppealStatus,
	}

	// Logic for T1 -> T2
	if currentTier == "individual" {
		eligibility["nextTier"] = "verified"
		daysActive := int(time.Since(acc.ActivatedAt).Hours() / 24)

		reqs := []gin.H{
			{"label": "30+ Days Active", "target": 30, "current": daysActive, "met": daysActive >= 30},
			{"label": "20+ Successful Orders", "target": 20, "current": acc.TotalOrders, "met": acc.TotalOrders >= 20},
			{"label": "No Disputes", "target": 0, "current": acc.DisputeCount, "met": acc.DisputeCount == 0},
		}

		metCount := 0
		for _, r := range reqs {
			if r["met"].(bool) {
				metCount++
			}
		}

		eligibility["requirements"] = reqs
		eligibility["progress"] = (metCount * 100) / len(reqs)
		eligibility["canUpgrade"] = metCount == len(reqs)
	}

	// Logic for T2 -> T3
	if currentTier == "verified" {
		eligibility["nextTier"] = "business"
		daysActive := int(time.Since(acc.ActivatedAt).Hours() / 24)

		reqs := []gin.H{
			{"label": "90+ Days Active", "target": 90, "current": daysActive, "met": daysActive >= 90},
			{"label": "100+ Successful Orders", "target": 100, "current": acc.TotalOrders, "met": acc.TotalOrders >= 100},
			{"label": "Under 2% Dispute Rate", "target": 2, "current": 0, "met": true}, // Simplified for now
		}

		metCount := 0
		for _, r := range reqs {
			if r["met"].(bool) {
				metCount++
			}
		}

		eligibility["requirements"] = reqs
		eligibility["progress"] = (metCount * 100) / len(reqs)
		eligibility["canUpgrade"] = metCount == len(reqs)
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Eligibility calculated", eligibility))
}

func (h *TierHandler) SubmitAppeal(c *gin.Context) {
	userIdStr, _ := c.Get("userId")
	vendorID, _ := primitive.ObjectIDFromHex(userIdStr.(string))

	var input struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Reason is required"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	user, err := h.UserRepo.GetByID(ctx, vendorID)
	if err != nil || user.VendorAccount == nil {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("Vendor account not found"))
		return
	}

	if user.VendorAccount.Status != "suspended" {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Account is not suspended"))
		return
	}

	err = h.UserRepo.UpdateVendorAppeal(ctx, vendorID, "pending", input.Reason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to submit appeal"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Appeal submitted successfully. Our team will review your case.", nil))
}
