package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/developia-II/ecommerce-backend/internal/adapters/repository"
	"github.com/developia-II/ecommerce-backend/internal/models"
	"github.com/developia-II/ecommerce-backend/utils"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type WalletHandler struct {
	Repo repository.TransactionRepository
	DB   *mongo.Database
}

func NewWalletHandler(db *mongo.Database) *WalletHandler {
	repo := repository.NewTransactionRepository(db)
	return &WalletHandler{
		Repo: repo,
		DB:   db,
	}
}

func (h *WalletHandler) GetWalletOverview(c *gin.Context) {
	userIdStr, _ := c.Get("userId")
	userID, _ := primitive.ObjectIDFromHex(userIdStr.(string))

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// 0. Maturate any pending funds for this vendor
	_ = h.Repo.MaturateFunds(ctx, userID)

	// 1. Get Balance
	account, err := h.Repo.GetBalance(ctx, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("Vendor wallet not found"))
		return
	}

	// 2. Get Recent Transactions
	transactions, err := h.Repo.GetTransactions(ctx, userID, 10)
	if err != nil {
		transactions = []models.Transaction{}
	}

	// 3. Get Payout History
	payouts, err := h.Repo.GetPayouts(ctx, userID)
	if err != nil {
		payouts = []models.PayoutRequest{}
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Wallet data fetched successfully", gin.H{
		"balance": gin.H{
			"available": account.AvailableBalance,
			"pending":   account.PendingBalance,
			"lifetime":  account.LifeTimeEarnings,
			"tier":      account.Tier,
			"holdDays":  account.PayoutHoldDays,
		},
		"transactions": transactions,
		"payouts":      payouts,
	}))
}

func (h *WalletHandler) RequestPayout(c *gin.Context) {
	userIdStr, _ := c.Get("userId")
	userID, _ := primitive.ObjectIDFromHex(userIdStr.(string))

	var input struct {
		Amount         float64           `json:"amount" binding:"required,gt=0"`
		Method         string            `json:"method" binding:"required"`
		AccountDetails map[string]string `json:"accountDetails" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request body"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// 1. Check Payout Eligibility (Tier check + Balance check)
	eligible, err := utils.CheckPayoutEligibility(ctx, userID, input.Amount, h.DB)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse(err.Error()))
		return
	}
	if !eligible {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Payout request rejected due to tier limits or insufficient funds"))
		return
	}

	// 2. Create Payout Request
	payout := models.PayoutRequest{
		ID:             primitive.NewObjectID(),
		VendorID:       userID,
		Amount:         input.Amount,
		Status:         "pending",
		Method:         input.Method,
		AccountDetails: input.AccountDetails,
		RequestedAt:    time.Now(),
		Reference:      fmt.Sprintf("WD-%d", time.Now().Unix()),
	}

	if err := h.Repo.RequestPayout(ctx, payout); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to process payout request"))
		return
	}

	// 3. Mock Email & Invoice Dispatch (Production ready for SendGrid/Resend)
	fmt.Printf("[Email Service Mock] Sending withdrawal receipt & invoice to vendor %s for $%.2f\n", userID.Hex(), input.Amount)

	c.JSON(http.StatusCreated, utils.SuccessResponse("Withdrawal successful", gin.H{
		"payout": payout,
	}))
}
