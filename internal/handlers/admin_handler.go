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
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type AdminHandler struct {
	DB       *mongo.Database
	TierRepo repository.TierRepository
}

func NewAdminHandler(db *mongo.Database) *AdminHandler {
	return &AdminHandler{
		DB:       db,
		TierRepo: repository.NewTierRepository(db),
	}
}

// GetPlatformStats returns a snapshot of key platform metrics.
func (h *AdminHandler) GetPlatformStats(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	usersCol := h.DB.Collection("users")
	ordersCol := h.DB.Collection("orders")
	vendorAccountsCol := h.DB.Collection("vendorAccounts")
	tierRequestsCol := h.DB.Collection("tierUpgradeRequests")

	totalUsers, _ := usersCol.CountDocuments(ctx, bson.M{"role": bson.M{"$ne": "admin"}})
	totalVendors, _ := vendorAccountsCol.CountDocuments(ctx, bson.M{})
	totalOrders, _ := ordersCol.CountDocuments(ctx, bson.M{})
	pendingTierRequests, _ := tierRequestsCol.CountDocuments(ctx, bson.M{"status": "pending"})

	// Total platform revenue from paid orders (sum of total field)
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"status": "paid"}}},
		{{Key: "$group", Value: bson.M{"_id": nil, "totalRevenue": bson.M{"$sum": "$total"}}}},
	}
	cursor, _ := ordersCol.Aggregate(ctx, pipeline)
	var revenueResult []struct {
		TotalRevenue float64 `bson:"totalRevenue"`
	}
	cursor.All(ctx, &revenueResult)

	totalRevenue := 0.0
	if len(revenueResult) > 0 {
		totalRevenue = revenueResult[0].TotalRevenue
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Stats fetched", gin.H{
		"totalUsers":          totalUsers,
		"totalVendors":        totalVendors,
		"totalOrders":         totalOrders,
		"totalRevenue":        totalRevenue,
		"pendingTierRequests": pendingTierRequests,
	}))
}

// ListVendors returns all vendor accounts with associated user info.
func (h *AdminHandler) ListVendors(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	statusFilter := c.Query("status") // active | suspended | all

	filter := bson.M{}
	if statusFilter != "" && statusFilter != "all" {
		filter["status"] = statusFilter
	}

	opts := options.Find().SetSort(bson.M{"createdAt": -1})
	cursor, err := h.DB.Collection("vendorAccounts").Find(ctx, filter, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to fetch vendors"))
		return
	}
	defer cursor.Close(ctx)

	var vendors []models.VendorAccount
	if err := cursor.All(ctx, &vendors); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to parse vendors"))
		return
	}

	// Enrich vendor accounts with store name, business type, and user profile
	var enrichedVendors []gin.H
	sellerAppsCol := h.DB.Collection("sellerApplications")
	usersCol := h.DB.Collection("users")

	for _, v := range vendors {
		var app models.SellerApplication
		var user models.User

		// Fetch User Profile
		_ = usersCol.FindOne(ctx, bson.M{"_id": v.UserID}).Decode(&user)

		// Fetch Application details
		filter := bson.M{"_id": v.ApplicationID}
		if v.ApplicationID.IsZero() {
			filter = bson.M{"userID": v.UserID}
		}
		_ = sellerAppsCol.FindOne(ctx, filter).Decode(&app)

		storeName := "Unnamed Store"
		businessType := "Independent"

		if app.StoreName != "" {
			storeName = app.StoreName
		} else if user.Name != "" {
			// Fallback to real name if store name isn't filled yet (e.g. Individual tier)
			storeName = user.Name
		}

		if app.BusinessTypeInfo != nil && app.BusinessTypeInfo.BusinessType != "" {
			businessType = app.BusinessTypeInfo.BusinessType
		}

		enrichedVendors = append(enrichedVendors, gin.H{
			"id":                  v.ID,
			"userID":              v.UserID,
			"userName":            user.Name,
			"userEmail":           user.Email,
			"createdAt":           v.CreatedAt,
			"tier":                v.Tier,
			"status":              v.Status,
			"maxProducts":         v.MaxProducts,
			"maxMonthlySales":     v.MaxMonthlySales,
			"transactionFee":      v.TransactionFee,
			"payoutHoldDays":      v.PayoutHoldDays,
			"lifeTimeEarnings":    v.LifeTimeEarnings,
			"currentMonthSales":   v.CurrentMonthSales,
			"totalOrders":         v.TotalOrders,
			"suspendedUntil":      v.SuspendedUntil,
			"verificationRetries": v.VerificationRetries,
			"storeName":           storeName,
			"businessType":        businessType,
		})
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Vendors fetched", gin.H{"vendors": enrichedVendors}))
}

// GetVendor returns full details for a single vendor by UserID or VendorAccountID.
func (h *AdminHandler) GetVendor(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	idParam := c.Param("id")
	objID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid vendor ID format"))
		return
	}

	// Attempt to find by VendorAccount ID or UserID
	var vendor models.VendorAccount
	filter := bson.M{
		"$or": []bson.M{
			{"_id": objID},
			{"userID": objID},
		},
	}

	if err := h.DB.Collection("vendorAccounts").FindOne(ctx, filter).Decode(&vendor); err != nil {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("Vendor not found"))
		return
	}

	// Fetch linked user details
	var user models.User
	_ = h.DB.Collection("users").FindOne(ctx, bson.M{"_id": vendor.UserID}).Decode(&user)

	// Fetch linked application data
	var app models.SellerApplication
	appFilter := bson.M{"_id": vendor.ApplicationID}
	if vendor.ApplicationID.IsZero() {
		appFilter = bson.M{"userID": vendor.UserID}
	}
	_ = h.DB.Collection("sellerApplications").FindOne(ctx, appFilter).Decode(&app)

	storeName := "Unnamed Store"
	businessType := "Independent"

	if app.StoreName != "" {
		storeName = app.StoreName
	} else if user.Name != "" {
		storeName = user.Name
	}

	if app.BusinessTypeInfo != nil && app.BusinessTypeInfo.BusinessType != "" {
		businessType = app.BusinessTypeInfo.BusinessType
	}

	// Construct comprehensive view
	details := gin.H{
		"account": vendor,
		"profile": gin.H{
			"userName":     user.Name,
			"userEmail":    user.Email,
			"userPhone":    user.Phone,
			"storeName":    storeName,
			"businessType": businessType,
		},
		"application": app,
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Vendor details fetched", details))
}

// ListProducts returns all products across all vendors with filtering.
func (h *AdminHandler) ListProducts(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	statusFilter := c.Query("status") // active | draft | flagged | all
	search := c.Query("search")

	filter := bson.M{}
	if statusFilter != "" && statusFilter != "all" {
		filter["status"] = statusFilter
	}
	if search != "" {
		filter["name"] = bson.M{"$regex": search, "$options": "i"}
	}

	opts := options.Find().SetSort(bson.M{"createdAt": -1})
	cursor, err := h.DB.Collection("products").Find(ctx, filter, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to fetch products"))
		return
	}
	defer cursor.Close(ctx)

	var products []models.Product
	cursor.All(ctx, &products)

	c.JSON(http.StatusOK, utils.SuccessResponse("Products fetched", gin.H{"products": products}))
}

// GetProduct returns full details for a single product contextually for the admin.
func (h *AdminHandler) GetProduct(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	idParam := c.Param("id")
	objID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid product ID format"))
		return
	}

	var product models.Product
	if err := h.DB.Collection("products").FindOne(ctx, bson.M{"_id": objID}).Decode(&product); err != nil {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("Product not found"))
		return
	}

	var vendor models.VendorAccount
	h.DB.Collection("vendorAccounts").FindOne(ctx, bson.M{"userID": product.VendorID}).Decode(&vendor)

	var app models.SellerApplication
	appFilter := bson.M{"_id": vendor.ApplicationID}
	if vendor.ApplicationID.IsZero() {
		appFilter = bson.M{"userID": vendor.UserID}
	}
	h.DB.Collection("sellerApplications").FindOne(ctx, appFilter).Decode(&app)
	
	var user models.User
	h.DB.Collection("users").FindOne(ctx, bson.M{"_id": vendor.UserID}).Decode(&user)

	storeName := app.StoreName
	if storeName == "" && user.Name != "" {
		storeName = user.Name
	}
	if storeName == "" {
		storeName = "Unnamed Store"
	}

	details := gin.H{
		"product": product,
		"vendor": gin.H{
			"id": vendor.ID,
			"userID": vendor.UserID,
			"userName": user.Name,
			"userEmail": user.Email,
			"storeName": storeName,
			"tier": vendor.Tier,
			"status": vendor.Status,
		},
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Product details fetched", details))
}

// FlagProduct allows an admin to hide/flag a product from the marketplace.
func (h *AdminHandler) FlagProduct(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	idParam := c.Param("id")
	objID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid product ID format"))
		return
	}

	update := bson.M{"$set": bson.M{"status": "flagged", "updatedAt": time.Now()}}
	res, err := h.DB.Collection("products").UpdateByID(ctx, objID, update)
	if err != nil || res.ModifiedCount == 0 {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to flag product or already flagged"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Product successfully flagged", nil))
}

// ApproveProduct allows an admin to unflag/approve a product back into the marketplace.
func (h *AdminHandler) ApproveProduct(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	idParam := c.Param("id")
	objID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid product ID format"))
		return
	}

	update := bson.M{"$set": bson.M{"status": "active", "updatedAt": time.Now()}}
	res, err := h.DB.Collection("products").UpdateByID(ctx, objID, update)
	if err != nil || res.ModifiedCount == 0 {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to approve product or already active"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Product successfully approved", nil))
}

// ListCustomers returns all users with the 'buyer' role.
func (h *AdminHandler) ListCustomers(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	search := c.Query("search")

	filter := bson.M{"role": "buyer"}
	if search != "" {
		filter["$or"] = []bson.M{
			{"name": bson.M{"$regex": search, "$options": "i"}},
			{"email": bson.M{"$regex": search, "$options": "i"}},
		}
	}

	opts := options.Find().SetSort(bson.M{"createdAt": -1})
	cursor, err := h.DB.Collection("users").Find(ctx, filter, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to fetch customers"))
		return
	}
	defer cursor.Close(ctx)

	var customers []models.User
	cursor.All(ctx, &customers)

	c.JSON(http.StatusOK, utils.SuccessResponse("Customers fetched", gin.H{"customers": customers}))
}

// ListOrders returns all orders on the platform with filtering.
func (h *AdminHandler) ListOrders(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	statusFilter := c.Query("status") // pending | paid | shipped | etc.

	filter := bson.M{}
	if statusFilter != "" && statusFilter != "all" {
		filter["status"] = statusFilter
	}

	opts := options.Find().SetSort(bson.M{"createdAt": -1})
	cursor, err := h.DB.Collection("orders").Find(ctx, filter, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to fetch orders"))
		return
	}
	defer cursor.Close(ctx)

	var orders []models.Order
	cursor.All(ctx, &orders)

	// Enrich orders with buyer name
	var enrichedOrders []gin.H
	usersCol := h.DB.Collection("users")

	for _, o := range orders {
		var user models.User
		_ = usersCol.FindOne(ctx, bson.M{"_id": o.UserID}).Decode(&user)
		
		enrichedOrders = append(enrichedOrders, gin.H{
			"id": o.ID,
			"orderNumber": o.OrderNumber,
			"buyerId": o.UserID,
			"buyerName": user.Name,
			"items": o.Items,
			"subTotal": o.Subtotal,
			"tax": o.Tax,
			"shippingFee": o.ShippingFee,
			"total": o.Total,
			"status": o.Status,
			"paymentMethod": o.PaymentMethod,
			"paymentStatus": o.PaymentStatus,
			"shippingAddress": o.ShippingAddress,
			"trackingNumber": o.TrackingNumber,
			"paymentId": o.PaymentID,
			"createdAt": o.CreatedAt,
			"updatedAt": o.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Orders fetched", gin.H{"orders": enrichedOrders}))
}

// GetOrder returns a single order's details including buyer info.
func (h *AdminHandler) GetOrder(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	idParam := c.Param("id")
	objID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid order ID format"))
		return
	}

	var order models.Order
	if err := h.DB.Collection("orders").FindOne(ctx, bson.M{"_id": objID}).Decode(&order); err != nil {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("Order not found"))
		return
	}

	var user models.User
	_ = h.DB.Collection("users").FindOne(ctx, bson.M{"_id": order.UserID}).Decode(&user)

	enrichedOrder := gin.H{
		"id": order.ID,
		"orderNumber": order.OrderNumber,
		"buyerId": order.UserID,
		"buyerName": user.Name,
		"buyerEmail": user.Email,
		"buyerPhone": user.Phone,
		"items": order.Items,
		"subTotal": order.Subtotal,
		"tax": order.Tax,
		"shippingFee": order.ShippingFee,
		"total": order.Total,
		"status": order.Status,
		"paymentMethod": order.PaymentMethod,
		"paymentStatus": order.PaymentStatus,
		"shippingAddress": order.ShippingAddress,
		"trackingNumber": order.TrackingNumber,
		"paymentId": order.PaymentID,
		"createdAt": order.CreatedAt,
		"updatedAt": order.UpdatedAt,
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Order details fetched", gin.H{"order": enrichedOrder}))
}

// ListTierRequests returns all tier upgrade requests.
func (h *AdminHandler) ListTierRequests(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	statusFilter := c.Query("status") // pending | approved | rejected | all

	filter := bson.M{}
	if statusFilter != "" && statusFilter != "all" {
		filter["status"] = statusFilter
	} else if statusFilter == "" {
		// Default: show pending first
		filter["status"] = models.UpgradeStatusPending
	}

	opts := options.Find().SetSort(bson.M{"createdAt": -1})
	cursor, err := h.DB.Collection("tierUpgradeRequests").Find(ctx, filter, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to fetch tier requests"))
		return
	}
	defer cursor.Close(ctx)

	var requests []models.TierUpgradeRequest
	cursor.All(ctx, &requests)

	c.JSON(http.StatusOK, utils.SuccessResponse("Tier requests fetched", gin.H{"requests": requests}))
}

// ApproveTierRequest approves a pending tier upgrade and updates the vendor's account.
func (h *AdminHandler) ApproveTierRequest(c *gin.Context) {
	reqID, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request ID"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// 1. Get the request
	var req models.TierUpgradeRequest
	err = h.DB.Collection("tierUpgradeRequests").FindOne(ctx, bson.M{"_id": reqID}).Decode(&req)
	if err != nil {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("Upgrade request not found"))
		return
	}

	if req.Status != models.UpgradeStatusPending {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Request is no longer pending"))
		return
	}

	// 2. Determine new limits based on target tier
	tierConfig := getTierConfig(req.RequestedTier)

	// 3. Update vendor account tier
	now := time.Now()
	_, err = h.DB.Collection("vendorAccounts").UpdateOne(ctx,
		bson.M{"userID": req.VendorID},
		bson.M{"$set": bson.M{
			"tier":            req.RequestedTier,
			"maxProducts":     tierConfig.MaxProducts,
			"maxMonthlySales": tierConfig.MaxMonthlySales,
			"transactionFee":  tierConfig.TransactionFee,
			"payoutHoldDays":  tierConfig.PayoutHoldDays,
			"tierUpgradedAt":  now,
			"updatedAt":       now,
		}},
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to update vendor tier"))
		return
	}

	// 4. Mark request as approved
	_, err = h.DB.Collection("tierUpgradeRequests").UpdateOne(ctx,
		bson.M{"_id": reqID},
		bson.M{"$set": bson.M{
			"status":     models.UpgradeStatusApproved,
			"reviewedAt": now,
			"updatedAt":  now,
		}},
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to mark request as approved"))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse(fmt.Sprintf("Vendor upgraded to %s tier successfully", req.RequestedTier), gin.H{
		"newTier":    req.RequestedTier,
		"holdDays":   tierConfig.PayoutHoldDays,
		"maxMonthly": tierConfig.MaxMonthlySales,
	}))
}

// RejectTierRequest rejects a pending tier upgrade request.
func (h *AdminHandler) RejectTierRequest(c *gin.Context) {
	reqID, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request ID"))
		return
	}

	var input struct {
		Reason string `json:"reason"`
	}
	c.ShouldBindJSON(&input)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// 1. Get the request
	var req models.TierUpgradeRequest
	err = h.DB.Collection("tierUpgradeRequests").FindOne(ctx, bson.M{"_id": reqID}).Decode(&req)
	if err != nil {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("Upgrade request not found"))
		return
	}

	if req.Status != models.UpgradeStatusPending {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Request is no longer pending"))
		return
	}

	now := time.Now()
	_, err = h.DB.Collection("tierUpgradeRequests").UpdateOne(ctx,
		bson.M{"_id": reqID},
		bson.M{"$set": bson.M{
			"status":     models.UpgradeStatusRejected,
			"adminNotes": input.Reason,
			"reviewedAt": now,
			"updatedAt":  now,
		}},
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to reject request"))
		return
	}

	// 2. Increment verification retries on VendorAccount
	var vendorAcc struct {
		VerificationRetries int `bson:"verificationRetries"`
	}
	h.DB.Collection("vendorAccounts").FindOne(ctx, bson.M{"userID": req.VendorID}).Decode(&vendorAcc)

	newRetries := vendorAcc.VerificationRetries + 1
	vendorUpdate := bson.M{
		"$set": bson.M{
			"verificationRetries": newRetries,
			"updatedAt":           now,
		},
	}

	isSuspended := false
	if newRetries >= 3 {
		suspendedUntil := now.Add(7 * 24 * time.Hour)
		vendorUpdate["$set"].(bson.M)["status"] = "suspended"
		vendorUpdate["$set"].(bson.M)["suspendedUntil"] = suspendedUntil
		isSuspended = true
	}

	h.DB.Collection("vendorAccounts").UpdateOne(ctx, bson.M{"userID": req.VendorID}, vendorUpdate)

	c.JSON(http.StatusOK, utils.SuccessResponse("Upgrade request rejected", gin.H{
		"retriesRemaining": 3 - newRetries,
		"isSuspended":      isSuspended,
	}))
}

// getTierConfig determines the account limits for a given tier name.
type tierLimits struct {
	MaxProducts     int
	MaxMonthlySales float64
	TransactionFee  float64
	PayoutHoldDays  int
}

func getTierConfig(tier string) tierLimits {
	switch tier {
	case "verified":
		return tierLimits{MaxProducts: 200, MaxMonthlySales: 25000, TransactionFee: 3.5, PayoutHoldDays: 5}
	case "business":
		return tierLimits{MaxProducts: 10000, MaxMonthlySales: 999999, TransactionFee: 2.0, PayoutHoldDays: 1}
	default: // individual
		return tierLimits{MaxProducts: 50, MaxMonthlySales: 5000, TransactionFee: 5.0, PayoutHoldDays: 7}
	}
}

func (h *AdminHandler) UnsuspendVendor(c *gin.Context) {
	vendorID, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid vendor ID"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	update := bson.M{
		"$set": bson.M{
			"status":              "active",
			"verificationRetries": 0,
			"appealStatus":        "approved",
			"updatedAt":           time.Now(),
		},
		"$unset": bson.M{
			"suspendedUntil": "",
		},
	}
	res, err := h.DB.Collection("vendorAccounts").UpdateOne(ctx, bson.M{"userID": vendorID}, update)
	if err != nil || res.MatchedCount == 0 {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("Vendor not found or could not update"))
		return
	}
	c.JSON(http.StatusOK, utils.SuccessResponse("Vendor account unsuspended successfully", nil))
}

func (h *AdminHandler) BanVendor(c *gin.Context) {
	vendorID, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid vendor ID"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	update := bson.M{
		"$set": bson.M{
			"status":       "banned",
			"appealStatus": "rejected",
			"updatedAt":    time.Now(),
		},
	}
	res, err := h.DB.Collection("vendorAccounts").UpdateOne(ctx, bson.M{"userID": vendorID}, update)
	if err != nil || res.MatchedCount == 0 {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("Vendor not found or could not update"))
		return
	}

	// Fetch user to send notification
	var user models.User
	err = h.DB.Collection("users").FindOne(ctx, bson.M{"_id": vendorID}).Decode(&user)
	if err == nil && user.Email != "" {
		subject := "Your Vendora Account Has Been Banned"
		body := "<p>Hello " + user.Name + ",</p><p>We regret to inform you that your vendor account has been permanently banned from our platform due to multiple failed verification attempts and a rejected appeal.</p>"
		go utils.SendEmail(user.Email, subject, body)
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Vendor account banned successfully", nil))
}
