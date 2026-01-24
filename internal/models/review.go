package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Review struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProductID primitive.ObjectID `json:"productId" bson:"productId"`
	OrderID   primitive.ObjectID `json:"orderId" bson:"orderId"`
	UserID    primitive.ObjectID `json:"userId" bson:"userId"`
	VendorID  primitive.ObjectID `json:"vendorId" bson:"vendorId"`

	UserName  string `json:"userName" bson:"userName"`
	UserImage string `json:"userImage" bson:"userImage"`

	Rating  int      `json:"rating" bson:"rating" validate:"required,min=1,max=5"`
	Comment string   `json:"comment" bson:"comment" validate:"required"`
	Images  []string `json:"images,omitempty" bson:"images,omitempty"`

	// Vendor Response
	Response   string     `json:"response,omitempty" bson:"response,omitempty"`
	ResponseAt *time.Time `json:"responseAt,omitempty" bson:"responseAt,omitempty"`

	CreatedAt time.Time `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt" bson:"updatedAt"`
}

type CreateReviewInput struct {
	ProductID primitive.ObjectID `json:"productId" binding:"required"`
	OrderID   primitive.ObjectID `json:"orderId" binding:"required"`
	Rating    int                `json:"rating" binding:"required,min=1,max=5"`
	Comment   string             `json:"comment" binding:"required"`
	Images    []string           `json:"images"`
}

type VendorResponseInput struct {
	Response string `json:"response" binding:"required"`
}
