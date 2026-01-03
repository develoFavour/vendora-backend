package main

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Run this script once to create database indexes
// Usage: go run scripts/create_indexes.go
func main() {
	// Increase timeout for cloud connection (Atlas is slower than localhost)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Connect to MongoDB Atlas
	mongoURI := "mongodb+srv://vendora:jebCkbDHuCgbvyCy@cluster0.um23o.mongodb.net/"
	clientOptions := options.Client().ApplyURI(mongoURI).SetServerSelectionTimeout(30 * time.Second)

	log.Println("🔄 Connecting to MongoDB Atlas...")
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatalf("❌ Failed to create client: %v", err)
	}
	defer client.Disconnect(ctx)

	// Verify connection works
	log.Println("🔄 Verifying connection...")
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("❌ Failed to connect to MongoDB Atlas: %v\nCheck your connection string and network access in MongoDB Atlas", err)
	}
	log.Println("✅ Connected to MongoDB Atlas successfully!")

	db := client.Database("vendora")

	// ========================================
	// PRODUCTS COLLECTION INDEXES
	// ========================================
	productsCollection := db.Collection("products")

	// 1. Index on vendorId for fast product count queries
	_, err = productsCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "vendorId", Value: 1}},
		Options: options.Index().SetName("idx_vendorId"),
	})
	if err != nil {
		log.Printf("Failed to create vendorId index: %v", err)
	} else {
		log.Println("✅ Created index: idx_vendorId on products.vendorId")
	}

	// 2. Compound index for vendor products listing (vendorId + createdAt)
	_, err = productsCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "vendorId", Value: 1},
			{Key: "createdAt", Value: -1}, // -1 for descending (newest first)
		},
		Options: options.Index().SetName("idx_vendor_products_date"),
	})
	if err != nil {
		log.Printf("Failed to create vendor_products_date index: %v", err)
	} else {
		log.Println("✅ Created compound index: idx_vendor_products_date")
	}

	// 3. Index on status for filtering (draft/active/archived)
	_, err = productsCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "status", Value: 1}},
		Options: options.Index().SetName("idx_status"),
	})
	if err != nil {
		log.Printf("Failed to create status index: %v", err)
	} else {
		log.Println("✅ Created index: idx_status on products.status")
	}

	// 4. Index on SEO slug for fast product page lookups
	_, err = productsCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "seo.slug", Value: 1}},
		Options: options.Index().SetName("idx_seo_slug").SetUnique(true),
	})
	if err != nil {
		log.Printf("Failed to create seo_slug index: %v", err)
	} else {
		log.Println("✅ Created unique index: idx_seo_slug on products.seo.slug")
	}

	// ========================================
	// VENDOR_ACCOUNTS COLLECTION INDEXES
	// ========================================
	vendorCollection := db.Collection("vendorAccounts")

	// 1. Index on userID for fast vendor lookups
	_, err = vendorCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "userID", Value: 1}},
		Options: options.Index().SetName("idx_userID").SetUnique(true),
	})
	if err != nil {
		log.Printf("Failed to create userID index: %v", err)
	} else {
		log.Println("✅ Created unique index: idx_userID on vendorAccounts.userID")
	}

	// 2. Index on status for admin filtering
	_, err = vendorCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "status", Value: 1}},
		Options: options.Index().SetName("idx_vendor_status"),
	})
	if err != nil {
		log.Printf("Failed to create vendor_status index: %v", err)
	} else {
		log.Println("✅ Created index: idx_vendor_status on vendorAccounts.status")
	}

	// 3. Index on tier for analytics
	_, err = vendorCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "tier", Value: 1}},
		Options: options.Index().SetName("idx_tier"),
	})
	if err != nil {
		log.Printf("Failed to create tier index: %v", err)
	} else {
		log.Println("✅ Created index: idx_tier on vendorAccounts.tier")
	}

	log.Println("\n🎉 All indexes created successfully!")
	log.Println("Run 'db.products.getIndexes()' and 'db.vendorAccounts.getIndexes()' in MongoDB shell to verify")
}
