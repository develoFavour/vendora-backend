package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TransactionType string

const (
	TransactionTypeSale       TransactionType = "sale"       // Money coming in from a sale
	TransactionTypePayout     TransactionType = "payout"     // Money being withdrawn by vendor
	TransactionTypeRefund     TransactionType = "refund"     // Money being returned to buyer
	TransactionTypeFee        TransactionType = "fee"        // Platform fee
	TransactionTypeAdjustment TransactionType = "adjustment" // Manual adjustment
)

type TransactionStatus string

const (
	TransactionStatusPending   TransactionStatus = "pending"   // Funds are held
	TransactionStatusAvailable TransactionStatus = "available" // Funds can be withdrawn
	TransactionStatusCompleted TransactionStatus = "completed" // Payout successful
	TransactionStatusFailed    TransactionStatus = "failed"    // Transaction failed
)

type Transaction struct {
	ID        primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	VendorID  primitive.ObjectID  `bson:"vendorId" json:"vendorId"`                   // The vendor receiving/sending money
	OrderID   *primitive.ObjectID `bson:"orderId,omitempty" json:"orderId,omitempty"` // Link to order if applicable
	Type      TransactionType     `bson:"type" json:"type"`
	Status    TransactionStatus   `bson:"status" json:"status"`
	Amount    float64             `bson:"amount" json:"amount"` // Net amount (could be negative for payouts)
	Fee       float64             `bson:"fee" json:"fee"`       // Platform fee taken
	Currency  string              `bson:"currency" json:"currency"`
	Reference string              `bson:"reference" json:"reference"` // External reference or description

	// Hold Logic
	HoldUntil *time.Time `bson:"holdUntil,omitempty" json:"holdUntil,omitempty"` // When pending becomes available

	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt" json:"updatedAt"`
}

type PayoutRequest struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	VendorID       primitive.ObjectID `bson:"vendorId" json:"vendorId"`
	Amount         float64            `bson:"amount" json:"amount"`
	Status         string             `bson:"status" json:"status"` // "pending", "approved", "rejected", "processed"
	Method         string             `bson:"method" json:"method"` // "bank_transfer", etc.
	AccountDetails map[string]string  `bson:"accountDetails" json:"accountDetails"`

	Reference  string `bson:"reference" json:"reference"`
	AdminNotes string `bson:"adminNotes,omitempty" json:"adminNotes,omitempty"`

	RequestedAt time.Time  `bson:"requestedAt" json:"requestedAt"`
	ProcessedAt *time.Time `bson:"processedAt,omitempty" json:"processedAt,omitempty"`
}
