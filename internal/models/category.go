package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Category struct {
	ID          primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	Name        string              `json:"name" bson:"name" validate:"required"`
	Description string              `json:"description,omitempty" bson:"description"`
	Slug        string              `json:"slug" bson:"slug" validate:"required"`
	ParentID    *primitive.ObjectID `json:"parentId,omitempty" bson:"parentId,omitempty"` // For subcategories
	Icon        string              `json:"icon,omitempty" bson:"icon,omitempty"`
	Image       string              `json:"image,omitempty" bson:"image,omitempty"`
	IsActive    bool                `json:"isActive" bson:"isActive" default:"true"`
	CreatedAt   time.Time           `json:"createdAt" bson:"createdAt"`
	UpdatedAt   time.Time           `json:"updatedAt" bson:"updatedAt"`
}

type UpdateCategoryInput struct {
	Name        *string             `json:"name,omitempty" bson:"name,omitempty"`
	Description *string             `json:"description,omitempty" bson:"description,omitempty"`
	Slug        *string             `json:"slug,omitempty" bson:"slug,omitempty"`
	ParentID    *primitive.ObjectID `json:"parentId,omitempty" bson:"parentId,omitempty"`
	Icon        *string             `json:"icon,omitempty" bson:"icon,omitempty"`
	Image       *string             `json:"image,omitempty" bson:"image,omitempty"`
	IsActive    *bool               `json:"isActive,omitempty" bson:"isActive,omitempty"`
	UpdatedAt   time.Time           `json:"updatedAt" bson:"updatedAt"`
}
