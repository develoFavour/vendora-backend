package domain

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Vendor represents the core business entity for a seller/vendor.
// Notice: We try to keep this struct clean from database-specific tags (bson) where possible,
// but for MongoDB it's often pragmatic to keep them.
// Ideally, we would map this to a separate "DAO" struct in the repository layer,
// but for this stage of mentorship, we will keep it simple.
type Vendor struct {
	ID                  primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	UserID              primitive.ObjectID `json:"userId" bson:"userID"`
	
	// Business Info
	BusinessName        string             `json:"businessName" bson:"businessName"`
	BusinessType        string             `json:"businessType" bson:"businessType"` // e.g., "sole-proprietor", "llc"
	Description         string             `json:"description" bson:"description"`
	Location            string             `json:"location" bson:"location"`
	Website             string             `json:"website,omitempty" bson:"website,omitempty"`
	
	// Store Setup
	StoreName           string             `json:"storeName" bson:"storeName"`
	StoreDescription    string             `json:"storeDescription" bson:"storeDescription"`
	StoreLogo           string             `json:"storeLogo,omitempty" bson:"storeLogo,omitempty"`
	
	// Verification
	Status              string             `json:"status" bson:"status"` // "draft", "pending", "approved", "rejected"
	TermsAccepted       bool               `json:"termsAccepted" bson:"termsAccepted"`
	TermsAcceptedAt     time.Time          `json:"termsAcceptedAt" bson:"termsAcceptedAt"`
	
	// Metadata
	CreatedAt           time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt           time.Time          `json:"updatedAt" bson:"updatedAt"`
	Version             int                `json:"version" bson:"version"`
}

// VendorRepository defines the interface for data access.
// The Service layer depends on this interface, not the concrete implementation.
type VendorRepository interface {
	// Create saves a new vendor application
	Create(ctx context.Context, vendor *Vendor) error
	
	// GetByUserID finds a vendor application by the user's ID
	GetByUserID(ctx context.Context, userID string) (*Vendor, error)
	
	// Update modifies an existing vendor application
	Update(ctx context.Context, vendor *Vendor) error
}

// VendorService defines the business logic interface.
// The Handler layer depends on this interface.
type VendorService interface {
	// SubmitApplication handles the full submission flow
	SubmitApplication(ctx context.Context, userID string, input SubmitApplicationInput) (*Vendor, error)
	
	// GetApplicationStatus retrieves the current status
	GetApplicationStatus(ctx context.Context, userID string) (*Vendor, error)
}

// SubmitApplicationInput is a DTO (Data Transfer Object) for the service layer.
// It decouples the service from the HTTP request body.
type SubmitApplicationInput struct {
	BusinessName     string
	BusinessType     string
	Description      string
	Location         string
	StoreName        string
	StoreDescription string
	TermsAccepted    bool
}
