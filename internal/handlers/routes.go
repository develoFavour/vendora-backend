package handlers

import (
	"net/http"

	"github.com/developia-II/ecommerce-backend/internal/adapters/repository"
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
		productRepo := repository.NewProductRepository(db)
		productHandler := NewProductHandler(db, productRepo)
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

		// Public Product Routes
		publicProductGroup := router.Group("/api/v1/public/products")
		{
			publicProductGroup.GET("", productHandler.FetchProductsPublic)
			publicProductGroup.GET("/:id", productHandler.FetchProductsPublicById)
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

			// Category Routes
			categories := protected.Group("/categories")
			{
				categories.POST("", middleware.RoleMiddleware("admin"), categoryHandler.CreateProductCategory)
				categories.GET("", categoryHandler.GetAllProductCategories)
			}

			// Media Routes
			protected.POST("/upload", uploadHandler.UploadImage)

			// Order Routes
			orderHandler := NewOrderHandler(db)
			orders := protected.Group("/orders")
			{
				orders.POST("", orderHandler.PlaceOrder)
				orders.GET("", orderHandler.GetUserOrders)
				orders.GET("/:id", orderHandler.GetOrderById)
			}

			// Vendor Order Routes
			vendorOrders := protected.Group("/vendor/orders")
			vendorOrders.Use(middleware.RoleMiddleware("vendor", "seller"))
			{
				vendorOrders.GET("", orderHandler.GetVendorOrders)
				vendorOrders.GET("/stats", orderHandler.GetVendorStats)
				vendorOrders.PUT("/:id/status", orderHandler.UpdateVendorOrderStatus)
				// For now using the same detail handler, but in future might need specific vendor view
				vendorOrders.GET("/:id", orderHandler.GetOrderById)
			}

			// Wishlist Routes
			wishlistHandler := NewWishlistHandler(db)
			wishlists := protected.Group("/wishlist")
			{
				wishlists.POST("", wishlistHandler.AddToWishlist)
				wishlists.DELETE("/:id", wishlistHandler.RemoveFromWishlist)
				wishlists.GET("", wishlistHandler.GetWishlist)
			}

			// Payment Routes
			paymentHandler := NewPaymentHandler(db)
			payments := protected.Group("/payments")
			{
				payments.POST("/create-intent", paymentHandler.CreatePaymentIntent)
			}

			// Public Webhook
			router.POST("/api/v1/payments/webhook", paymentHandler.HandleWebhook)

			// Cart Routes
			cartHandler := NewCartHandler(db)
			carts := protected.Group("/cart")
			{
				carts.POST("", cartHandler.AddToCart)
				carts.DELETE("/:id", cartHandler.RemoveFromCart)
				carts.GET("", cartHandler.GetCart)
				carts.PUT("/:id", cartHandler.UpdateQuantity)
				carts.DELETE("", cartHandler.ClearCart)
			}
		}

	} else {
		logrus.Warn("Database not connected - running with limited functionality")
		router.Any("/api/*path", func(c *gin.Context) {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   "Database connection not available",
				"message": "The server is running but could not connect to the database. Please check server logs.",
			})
		})
	}
}
