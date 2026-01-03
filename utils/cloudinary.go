package utils

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

// UploadToCloudinary handles the streaming of a file to Cloudinary storage.
// It returns the secure URL of the uploaded image.
func UploadToCloudinary(file io.Reader, filename string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Initialize Cloudinary
	cld, err := cloudinary.NewFromParams(
		os.Getenv("CLOUDINARY_CLOUD_NAME"),
		os.Getenv("CLOUDINARY_API_KEY"),
		os.Getenv("CLOUDINARY_API_SECRET"),
	)
	if err != nil {
		return "", err
	}

	// 2. Perform the upload
	uniqueFilename := true
	uploadResult, err := cld.Upload.Upload(ctx, file, uploader.UploadParams{
		PublicID:       filename,
		Folder:         "vendora/products",
		UniqueFilename: &uniqueFilename,
	})
	if err != nil {
		return "", err
	}

	// 3. Return the secure URL
	return uploadResult.SecureURL, nil
}
