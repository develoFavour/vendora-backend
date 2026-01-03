package vendor

import (
	"context"

	"github.com/developia-II/ecommerce-backend/internal/models"
	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type Service interface {
	SaveDraft(ctx context.Context, userID primitive.ObjectID, role string, path string, data any) (models.UserOnboardingDraft, error)
	SubmitApplication(ctx context.Context, userID primitive.ObjectID) (primitive.ObjectID, error)
	Approve(ctx context.Context, appID primitive.ObjectID, adminID primitive.ObjectID) (*models.VendorAccount, error)
	Reject(ctx context.Context, appID primitive.ObjectID, adminID primitive.ObjectID, reason string) error
}

type service struct {
	db *mongo.Database
	v  *validator.Validate
}
