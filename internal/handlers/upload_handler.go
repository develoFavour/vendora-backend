package handlers

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/developia-II/ecommerce-backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/mongo"
)

type UploadHandler struct {
	DB *mongo.Database
}

func NewUploadHandler(db *mongo.Database) *UploadHandler {
	return &UploadHandler{DB: db}
}

// UploadImage handles the POST /api/v1/upload request.
// It validates the user, the file size, and the file content type before streaming to Cloudinary.
func (h *UploadHandler) UploadImage(c *gin.Context) {
	// 1. Guard the Door (Authentication)
	authHeader := c.Request.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("Invalid or missing token"))
		return
	}
	authStr := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := utils.VerifyToken(authStr)
	if err != nil {
		c.JSON(http.StatusForbidden, utils.ErrorResponse(err.Error()))
		return
	}

	// Only vendors are allowed to upload product images
	if claims.Role != "vendor" {
		c.JSON(http.StatusForbidden, utils.ErrorResponse("Only vendors can upload images"))
		return
	}

	// 2. Check the Weight (Size Limit: 10MB)
	// http.MaxBytesReader prevents the server from reading more than the limit
	const MaxUploadSize = 10 << 20 // 10MB
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxUploadSize)

	// 3. Grab the Package (Extracting the file)
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("No file provided or file too large (Max 10MB)"))
		return
	}
	defer file.Close()

	// 4. Scan the Contents (Magic Number Validation)
	// We read the first 512 bytes to detect the actual file type
	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to read file for validation"))
		return
	}
	// Reset the file pointer to the beginning so Cloudinary can read the whole file
	file.Seek(0, 0)

	contentType := http.DetectContentType(buffer)
	allowedTypes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/webp": true,
		"image/gif":  true,
	}

	if !allowedTypes[contentType] {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Unsupported file type. Please upload JPG, PNG, WEBP, or GIF"))
		return
	}

	// 5. Rename the Original (Security/Sanitization)
	// We use UUID to prevent path traversal and name collisions
	uniqueID := uuid.New().String()
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		// Fallback extension based on content type if original filename has none
		switch contentType {
		case "image/jpeg":
			ext = ".jpg"
		case "image/png":
			ext = ".png"
		case "image/webp":
			ext = ".webp"
		case "image/gif":
			ext = ".gif"
		}
	}
	safeFilename := fmt.Sprintf("%s%s", uniqueID, ext)

	// 6. Ship to Storage (Streaming to Cloudinary)
	imageUrl, err := utils.UploadToCloudinary(file, safeFilename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Cloudinary upload failed: "+err.Error()))
		return
	}

	// 7. Send the Receipt (Success Response)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Image uploaded successfully",
		"url":     imageUrl,
		"size":    header.Size,
		"type":    contentType,
	})
}
