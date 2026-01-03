package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ProductStatus string

const (
	ProductStatusDraft    ProductStatus = "draft"
	ProductStatusActive   ProductStatus = "active"
	ProductStatusArchived ProductStatus = "archived"
)

type Dimensions struct {
	Length float64 `json:"length" bson:"length"` // cm
	Width  float64 `json:"width" bson:"width"`   // cm
	Height float64 `json:"height" bson:"height"` // cm
	Weight float64 `json:"weight" bson:"weight"` // kg
}

type VariantOption struct {
	Name   string   `json:"name" bson:"name"`     // e.g., "Color", "Size"
	Values []string `json:"values" bson:"values"` // e.g., ["Red", "Blue"], ["S", "M"]
}

type Variant struct {
	ID         string            `json:"id" bson:"id"`
	SKU        string            `json:"sku" bson:"sku"`
	Price      float64           `json:"price" bson:"price"`
	Stock      int               `json:"stock" bson:"stock"`
	Options    map[string]string `json:"options" bson:"options"`       // e.g., {"Color": "Red", "Size": "M"}
	ImageIndex int               `json:"imageIndex" bson:"imageIndex"` // Index of the specific image for this variant
}

type SEO struct {
	Title       string   `json:"title" bson:"title"`
	Description string   `json:"description" bson:"description"`
	Keywords    []string `json:"keywords" bson:"keywords"`
	Slug        string   `json:"slug" bson:"slug"`
}

type Product struct {
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	VendorID primitive.ObjectID `json:"_" bson:"vendorId"`

	// Basic Info
	Name        string `json:"name" bson:"name" validate:"required"`
	Description string `json:"description" bson:"description"`
	Brand       string `json:"brand" bson:"brand"`

	// Categorization
	CategoryID     primitive.ObjectID   `json:"categoryId" bson:"categoryId"`
	SubCategoryIDs []primitive.ObjectID `json:"subCategoryIds" bson:"subCategoryIds"`
	Tags           []string             `json:"tags" bson:"tags"`

	// Media
	Images   []string `json:"images" bson:"images"` // First image is primary
	VideoURL string   `json:"videoUrl" bson:"videoUrl"`

	// Pricing
	Price     float64 `json:"price" bson:"price" validate:"required,gt=0"`
	SalePrice float64 `json:"salePrice" bson:"salePrice"`
	CostPrice float64 `json:"costPrice" bson:"costPrice"` // For analytics
	TaxRate   float64 `json:"taxRate" bson:"taxRate"`     // Percentage

	// Inventory
	SKU               string `json:"sku" bson:"sku"`
	Barcode           string `json:"barcode" bson:"barcode"` // ISBN, UPC, etc.
	Stock             int    `json:"stock" bson:"stock" validate:"gte=0"`
	LowStockThreshold int    `json:"lowStockThreshold" bson:"lowStockThreshold"`
	AllowBackorder    bool   `json:"allowBackorder" bson:"allowBackorder"`

	// Shipping & Delivery
	IsDigital     bool       `json:"isDigital" bson:"isDigital"`
	Dimensions    Dimensions `json:"dimensions" bson:"dimensions"`
	ShippingClass string     `json:"shippingClass" bson:"shippingClass"`

	// Variants
	HasVariants    bool            `json:"hasVariants" bson:"hasVariants"`
	VariantOptions []VariantOption `json:"variantOptions" bson:"variantOptions"`
	Variants       []Variant       `json:"variants" bson:"variants"`

	// SEO & Metadata
	SEO      SEO               `json:"seo" bson:"seo"`
	Metadata map[string]string `json:"metadata" bson:"metadata"`
	Status   ProductStatus     `json:"status" bson:"status" default:"draft"`

	// Analytics (Computed or Cached)
	Rating      float64 `json:"rating" bson:"rating"`
	ReviewCount int     `json:"reviewCount" bson:"reviewCount"`
	TotalSales  int     `json:"totalSales" bson:"totalSales"`

	CreatedAt time.Time `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt" bson:"updatedAt"`
}
type UpdateProductInput struct {
	Name        string             `json:"name" bson:"name"`
	Description string             `json:"description" bson:"description"`
	Price       float64            `json:"price" bson:"price"`
	Stock       int                `json:"stock" bson:"stock"`
	CategoryId  primitive.ObjectID `json:"categoryId" bson:"categoryId"`
	SEO         SEO                `json:"seo" bson:"seo"`
	SKU         string             `json:"sku" bson:"sku"`
	Metadata    map[string]string  `json:"metadata" bson:"metadata"`
	Status      ProductStatus      `json:"status" bson:"status"`
	UpdatedAt   time.Time          `json:"updatedAt" bson:"updatedAt"`

	SalePrice         float64         `json:"salePrice" bson:"salePrice"`
	CostPrice         float64         `json:"costPrice" bson:"costPrice"`
	TaxRate           float64         `json:"taxRate" bson:"taxRate"`
	Tags              []string        `json:"tags" bson:"tags"`
	Brand             string          `json:"brand" bson:"brand"`
	Dimensions        Dimensions      `json:"dimensions" bson:"dimensions"`
	ShippingClass     string          `json:"shippingClass" bson:"shippingClass"`
	IsDigital         bool            `json:"isDigital" bson:"isDigital"`
	LowStockThreshold int             `json:"lowStockThreshold" bson:"lowStockThreshold"`
	AllowBackorder    bool            `json:"allowBackorder" bson:"allowBackorder"`
	HasVariants       bool            `json:"hasVariants" bson:"hasVariants"`
	VariantOptions    []VariantOption `json:"variantOptions" bson:"variantOptions"`
	Variants          []Variant       `json:"variants" bson:"variants"`

	SubCategoryIds []primitive.ObjectID `json:"subCategoryIds" bson:"subCategoryIds"`
	Images         []string             `json:"images" bson:"images"`
	VideoURL       string               `json:"videoUrl" bson:"videoUrl"`
}
