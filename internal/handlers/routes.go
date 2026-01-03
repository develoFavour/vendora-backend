package handlers

import (
	"net/http"

	"github.com/developia-II/ecommerce-backend/internal/middleware"
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
		authHandler := NewAuthHandler(db)
		onboardingHandler := NewOnboardingHandler(db)
		productHandler := NewProductHandler(db)
		categoryHandler := NewCategoryHandler(db)
		uploadHandler := NewUploadHandler(db)

		// Public Routes
		authGroup := router.Group("/api/v1/auth")
		{
			authGroup.POST("/register", authHandler.CreateUser)
			authGroup.POST("/verify/:token", authHandler.VerifyEmail)
			authGroup.POST("/login", authHandler.LoginUser)
			authGroup.POST("/refresh-token", authHandler.RefreshToken)
			authGroup.POST("/forgot-password", authHandler.ForgotPassword)
			authGroup.POST("/reset-password", authHandler.ResetPassword)
			authGroup.POST("/resend/:token", authHandler.ResendVerification)
		}

		// Protected Routes
		protected := router.Group("/api/v1")
		protected.Use(middleware.AuthMiddleware())
		{
			// Onboarding Routes
			onboarding := protected.Group("/onboarding")
			{
				onboarding.POST("/interests", onboardingHandler.ClientUpdateInterest)
				onboarding.POST("/preference", onboardingHandler.ClientUpdatePreference)
				onboarding.POST("/profile", onboardingHandler.CompleteOnboardingFlow)
				onboarding.POST("/draft", onboardingHandler.UserOnboardingDraft)
				onboarding.GET("/draft", onboardingHandler.GetOnboardingDraft)

				seller := onboarding.Group("/seller")
				{
					seller.POST("/business-type", onboardingHandler.SellerBusinessType)
					seller.POST("/business-category", onboardingHandler.SellerBusinessCategory)
					seller.POST("/business-details", onboardingHandler.SellerBusinessInfo)
					seller.POST("/store-details", onboardingHandler.StoreDetails)
					seller.POST("/verification", onboardingHandler.SellerVerification)
				}
			}

			// Product Routes
			products := protected.Group("/products")
			{
				products.POST("", middleware.RoleMiddleware("vendor", "seller"), productHandler.CreateProduct)
				products.GET("", productHandler.GetVendorProducts)
				products.PUT("/:id", productHandler.UpdateProduct)
				products.GET("/:id", productHandler.GetProductById)
				products.DELETE("/:id", productHandler.DeleteProduct)
			}

			// Media Routes
			protected.POST("/upload", uploadHandler.UploadImage)

			// Category Routes
			categories := protected.Group("/categories")
			{
				categories.POST("", middleware.RoleMiddleware("admin"), categoryHandler.CreateProductCategory)
				categories.GET("", categoryHandler.GetAllProductCategories)
			}
		}

	} else {
		logrus.Warn("Database not connected - running with limited functionality")
	}
}
