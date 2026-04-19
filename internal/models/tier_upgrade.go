package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TierUpgradeStatus string

const (
	UpgradeStatusPending    TierUpgradeStatus = "pending"
	UpgradeStatusApproved   TierUpgradeStatus = "approved"
	UpgradeStatusRejected   TierUpgradeStatus = "rejected"
	UpgradeStatusRequesting TierUpgradeStatus = "requesting"
)

type TierUpgradeRequest struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	VendorID      primitive.ObjectID `bson:"vendorID" json:"vendorId"`
	CurrentTier   string             `bson:"currentTier" json:"currentTier"`
	RequestedTier string             `bson:"requestedTier" json:"requestedTier"`

	// Verification documents based on the requested tier
	Documents []VerificationDocument `bson:"documents" json:"documents"`

	// Business Info (if upgrading to Business Tier)
	BusinessInfo *BusinessDetails `bson:"businessInfo,omitempty" json:"businessInfo,omitempty"`

	Status      TierUpgradeStatus `bson:"status" json:"status"`
	RiskScore   int               `bson:"riskScore,omitempty" json:"riskScore,omitempty"`
	ReviewNotes string            `bson:"reviewNotes,omitempty" json:"reviewNotes,omitempty"`
	AdminNotes  string            `bson:"adminNotes,omitempty" json:"adminNotes,omitempty"`

	CreatedAt  time.Time  `bson:"createdAt" json:"createdAt"`
	UpdatedAt  time.Time  `bson:"updatedAt" json:"updatedAt"`
	ReviewedAt *time.Time `bson:"reviewedAt,omitempty" json:"reviewedAt,omitempty"`
}
