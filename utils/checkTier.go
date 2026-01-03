package utils

import (
	"context"
	"errors"
	"fmt"

	"github.com/developia-II/ecommerce-backend/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type LimitCheckResult struct {
	Allowed      bool
	CurrentCount int64
	MaxAllowed   int
	Tier         string
	UpgradeURL   string
}

func CheckVendorLimits(ctx context.Context, vendorID primitive.ObjectID, db *mongo.Database) (LimitCheckResult, error) {
	var vendor models.VendorAccount

	collection := db.Collection("vendorAccounts")
	filter := bson.M{"userID": vendorID}
	err := collection.FindOne(ctx, filter).Decode(&vendor)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return LimitCheckResult{}, errors.New("vendor account not found or inactive")
		}
		return LimitCheckResult{}, fmt.Errorf("failed to fetch vendor account: %w", err)
	}

	if vendor.Status != "active" {
		return LimitCheckResult{}, errors.New("account has been banned or suspended")
	}

	if vendor.ProductCount >= vendor.MaxProducts {
		return LimitCheckResult{
				Allowed:      false,
				CurrentCount: int64(vendor.ProductCount),
				MaxAllowed:   vendor.MaxProducts,
				Tier:         vendor.Tier,
				UpgradeURL:   "",
			}, fmt.Errorf(
				"you've reached your product limit (%d/%d) for %s tier. Upgrade your account to add more products",
				vendor.ProductCount,
				vendor.MaxProducts,
				vendor.Tier,
			)
	}
	return LimitCheckResult{
		Allowed:      true,
		CurrentCount: int64(vendor.ProductCount),
		MaxAllowed:   vendor.MaxProducts,
		Tier:         vendor.Tier,
		UpgradeURL:   "",
	}, nil

}
