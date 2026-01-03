package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
)

func SetupRoutes(router *gin.Engine, db *mongo.Database) {
	logrus.Info("Setting up routes...")

	router.GET("/", func(c *gin.Context) {
		logrus.Info("Handling request to /")
		c.JSON(http.StatusOK, gin.H{
			"message": "Server is running!",
			"status":  "ok",
		})
	})

	router.GET("/health", func(c *gin.Context) {
		logrus.Info("Handling request to /health")
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "vendora-backend",
		})
	})

	if db != nil {
		logrus.Info("Database connected - setting up database routes")
		router.POST("/api/v1/auth/register", NewAuthHandler(db).CreateUser)
		router.POST("/api/v1/auth/verify/:token", NewAuthHandler(db).VerifyEmail)
		router.POST("/api/v1/auth/login", NewAuthHandler(db).LoginUser)
		router.POST("/api/v1/auth/refresh-token", NewAuthHandler(db).RefreshToken)
		router.POST("/api/v1/auth/forgot-password", NewAuthHandler(db).ForgotPassword)
		router.POST("/api/v1/auth/reset-password", NewAuthHandler(db).ResetPassword)
		router.POST("/api/v1/onboarding/interests", NewOnboardingHandler(db).ClientUpdateInterest)
		router.POST("/api/v1/onboarding/preference", NewOnboardingHandler(db).ClientUpdatePreference)
		router.POST("/api/v1/onboarding/profile", NewOnboardingHandler(db).CompleteOnboardingFlow)
		router.POST("/api/v1/onboarding/draft", NewOnboardingHandler(db).UserOnboardingDraft)
		router.GET("/api/v1/onboarding/draft", NewOnboardingHandler(db).GetOnboardingDraft)
		router.POST("/api/v1/auth/resend/:token", NewAuthHandler(db).ResendVerification)
		router.POST("/api/v1/onboarding/seller/business-type", NewOnboardingHandler(db).SellerBusinessType)
		router.POST("/api/v1/onboarding/seller/business-category", NewOnboardingHandler(db).SellerBusinessCategory)
		router.POST("/api/v1/onboarding/seller/business-details", NewOnboardingHandler(db).SellerBusinessInfo)
		router.POST("/api/v1/onboarding/seller/store-details", NewOnboardingHandler(db).StoreDetails)
		router.POST("/api/v1/onboarding/seller/verification", NewOnboardingHandler(db).SellerVerification)

		// Product & Media Routes
		router.POST("/api/v1/products", NewProductHandler(db).CreateProduct)
		router.GET("/api/v1/products", NewProductHandler(db).GetVendorProducts)
		router.PUT("/api/v1/products/:id", NewProductHandler(db).UpdateProduct)
		router.POST("/api/v1/upload", NewUploadHandler(db).UploadImage)

		// Admin Route
		// Category Routes
		router.POST("/api/v1/categories", NewCategoryHandler(db).CreateProductCategory)
		router.GET("/api/v1/categories", NewCategoryHandler(db).GetAllProductCategories)

	} else {
		logrus.Warn("Database not connected - running with limited functionality")
	}
}
