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
		userRepo := repository.NewUserRepository(db)
		authHandler := NewAuthHandler(db)
		onboardingHandler := NewOnboardingHandler(db)
		productRepo := repository.NewProductRepository(db)
		productHandler := NewProductHandler(db, productRepo)
		categoryHandler := NewCategoryHandler(db)
		uploadHandler := NewUploadHandler(db)
		vendorHandler := NewVendorHandler(db, userRepo)

		// Public Routes
		v1Group := router.Group("/api/v1")
		authGroup := v1Group.Group("/auth")
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
		publicProductGroup := v1Group.Group("/public/products")
		{
			publicProductGroup.GET("", productHandler.FetchProductsPublic)
			publicProductGroup.GET("/:id", productHandler.FetchProductsPublicById)
			publicProductGroup.GET("/:id/similar", productHandler.FetchSimilarProducts)
		}

		// Public Category Routes
		publicCategoryGroup := v1Group.Group("/public/categories")
		{
			publicCategoryGroup.GET("", categoryHandler.GetAllProductCategories)
		}

		// Public Vendor Routes
		publicVendorGroup := v1Group.Group("/public/vendors")
		{
			publicVendorGroup.GET("", vendorHandler.ListPublicVendors)
			publicVendorGroup.GET("/:id", vendorHandler.GetPublicVendorById)
		}

		// Protected Routes
		protected := router.Group("/api/v1")
		protected.Use(middleware.AuthMiddleware())
		{
			// Profile / User Routes
			userHandler := NewUserHandler(db)
			profileGroup := protected.Group("/profile")
			{
				profileGroup.GET("", userHandler.GetProfile)
				profileGroup.PUT("", userHandler.UpdateProfile)
				profileGroup.PUT("/password", userHandler.ChangePassword)
			}

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
				categories.GET("", categoryHandler.GetAllProductCategories)
				categories.GET("/:id", categoryHandler.GetCategoryById)
				categories.POST("", middleware.RoleMiddleware("admin"), categoryHandler.CreateProductCategory)
				categories.PUT("/:id", middleware.RoleMiddleware("admin"), categoryHandler.UpdateProductCategory)
				categories.DELETE("/:id", middleware.RoleMiddleware("admin"), categoryHandler.DeleteProductCategory)
			}

			// Media Routes
			protected.POST("/upload", uploadHandler.UploadImage)

			// Order Routes
			orderHandler := NewOrderHandler(db)
			orders := protected.Group("/orders")
			{
				orders.POST("", orderHandler.PlaceOrder)
				orders.GET("", orderHandler.GetUserOrders)
				orders.GET("/overview", orderHandler.GetBuyerOverview)
				orders.GET("/:id", orderHandler.GetOrderById)
				orders.PUT("/:id/confirm-receipt", orderHandler.ConfirmReceipt)
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
				wishlists.GET("", wishlistHandler.GetWishlist)
				wishlists.DELETE("/:id", wishlistHandler.RemoveFromWishlist)
			}

			// Review Routes
			reviewHandler := NewReviewHandler(db)
			protected.POST("/reviews", reviewHandler.CreateReview)

			vendorReviews := protected.Group("/vendor/reviews")
			vendorReviews.Use(middleware.RoleMiddleware("vendor", "seller"))
			{
				vendorReviews.GET("", reviewHandler.GetVendorReviews)
				vendorReviews.POST("/:id/respond", reviewHandler.RespondToReview)
			}

			// Wallet & Payout Routes
			walletHandler := NewWalletHandler(db)
			wallet := protected.Group("/vendor/wallet")
			wallet.Use(middleware.RoleMiddleware("vendor", "seller"))
			{
				wallet.GET("/overview", walletHandler.GetWalletOverview)
				wallet.POST("/payout", walletHandler.RequestPayout)
			}

			// Tier Upgrade Routes
			tierHandler := NewTierHandler(db)
			tier := protected.Group("/vendor/tier")
			tier.Use(middleware.RoleMiddleware("vendor", "seller"))
			{
				tier.POST("/upgrade", tierHandler.RequestUpgrade)
				tier.GET("/status", tierHandler.GetUpgradeStatus)
				tier.GET("/history", tierHandler.GetUpgradeHistory)
				tier.GET("/eligibility", tierHandler.GetEligibility)
				tier.POST("/appeal", tierHandler.SubmitAppeal)
			}

			// Public Vendor Application
			protected.POST("/vendor/apply", vendorHandler.ApplyForVendor)

			// Admin Routes
			adminHandler := NewAdminHandler(db)
			admin := protected.Group("/admin")
			admin.Use(middleware.RoleMiddleware("admin"))
			{
				admin.GET("/stats", adminHandler.GetPlatformStats)
				admin.GET("/vendors", adminHandler.ListVendors)
				admin.GET("/vendors/:id", adminHandler.GetVendor)
				admin.GET("/products", adminHandler.ListProducts)
				admin.GET("/products/:id", adminHandler.GetProduct)
				admin.PUT("/products/:id/flag", adminHandler.FlagProduct)
				admin.PUT("/products/:id/approve", adminHandler.ApproveProduct)
				admin.GET("/customers", adminHandler.ListCustomers)
				admin.GET("/orders", adminHandler.ListOrders)
				admin.GET("/orders/:id", adminHandler.GetOrder)
				admin.GET("/tier-requests", adminHandler.ListTierRequests)
				admin.PUT("/tier-requests/:id/approve", adminHandler.ApproveTierRequest)
				admin.PUT("/tier-requests/:id/reject", adminHandler.RejectTierRequest)
				admin.PUT("/vendors/:id/unsuspend", adminHandler.UnsuspendVendor)
				admin.PUT("/vendors/:id/ban", adminHandler.BanVendor)
			}

			// Payment Routes
			paymentHandler := NewPaymentHandler(db)
			payments := protected.Group("/payments")
			{
				payments.POST("/create-intent", paymentHandler.CreatePaymentIntent)
				payments.POST("/verify/:id", paymentHandler.VerifyPayment)
			}

			// Public Webhook (Payment handler already initialized above)
			router.POST("/api/v1/payments/webhook", paymentHandler.HandleWebhook)

			// Public Review Routes
			v1Group.GET("/products/:id/reviews", reviewHandler.GetProductReviews)


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
