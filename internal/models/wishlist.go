package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Wishlist struct {
	ID         primitive.ObjectID   `bson:"_id,omitempty" json:"id"`
	UserID     primitive.ObjectID   `json:"userId" bson:"userId"`
	ProductIDs []primitive.ObjectID `json:"productIds" bson:"productIds"`
	CreatedAt  time.Time            `json:"createdAt" bson:"createdAt"`
}

type PopulatedWishlist struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID    primitive.ObjectID `json:"userId" bson:"userId"`
	Products  []Product          `json:"products" bson:"products"`
	CreatedAt time.Time          `json:"createdAt" bson:"createdAt"`
}
