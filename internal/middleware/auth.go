package middleware

import (
	"net/http"
	"strings"

	"github.com/developia-II/ecommerce-backend/utils"
	"github.com/gin-gonic/gin"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, utils.ErrorResponse("Authorization header is required"))
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, utils.ErrorResponse("Authorization header must be Bearer token"))
			c.Abort()
			return
		}

		tokenString := parts[1]
		claims, err := utils.VerifyToken(tokenString)
		if err != nil {
			// Return 401 Unauthorized for token errors to trigger frontend refresh
			c.JSON(http.StatusUnauthorized, utils.ErrorResponse(err.Error()))
			c.Abort()
			return
		}

		// Store claims in context
		c.Set("userId", claims.UserID)
		c.Set("role", claims.Role)
		c.Next()
	}
}

func RoleMiddleware(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			c.JSON(http.StatusUnauthorized, utils.ErrorResponse("Role not found in context"))
			c.Abort()
			return
		}

		userRole := role.(string)
		isAllowed := false
		for _, r := range allowedRoles {
			if strings.ToLower(userRole) == strings.ToLower(r) {
				isAllowed = true
				break
			}
		}

		if !isAllowed {
			c.JSON(http.StatusForbidden, utils.ErrorResponse("You do not have permission to access this resource"))
			c.Abort()
			return
		}

		c.Next()
	}
}
