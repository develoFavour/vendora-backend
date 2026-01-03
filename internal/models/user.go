package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RegisterInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	Name     string `json:"name" binding:"required"`
	Phone    string `json:"phone"`
	Address  string `json:"address"`
}
type User struct {
	ID               primitive.ObjectID `bson:"_id,omitempty"`
	Email            string             `json:"email" bson:"email" validate:"required,email"`
	Name             string             `json:"name" bson:"name"`
	Address          string             `json:"address" bson:"address"`
	Phone            string             `json:"phone" bson:"phone"`
	Password         string             `json:"-" bson:"password" validate:"required,min=6"`
	Role             string             `json:"role" bson:"role"`
	CreatedAt        time.Time          `json:"createdAt" bson:"createdAt"`
	UpdatedAt        time.Time          `json:"updatedAt" bson:"updatedAt"`
	IsVerified       bool               `json:"isverified" bson:"isverified"`
	ResetToken       string             `json:"-" bson:"resetToken,omitempty"`
	ResetTokenExpiry time.Time          `json:"-" bson:"resetTokenExpiry,omitempty"`
	PasswordResetAt  time.Time          `json:"-" bson:"passwordResetAt,omitempty"`

	RefreshToken        string    `json:"-" bson:"refreshToken,omitempty"`
	RefreshTokenExpiry  time.Time `json:"-" bson:"refreshTokenExpiry,omitempty"`
	OnboardingCompleted bool      `json:"onboardingCompleted" bson:"onboardingCompleted"`

	Preferences *UserPreferences `json:"preferences" bson:"preferences"`
	Interests   *UserInterests   `json:"interests" bson:"interests"`
	Profile     *UserProfile     `json:"profile" bson:"profile"`

	VendorStatus      string             `json:"vendorStatus" bson:"vendorStatus"` // "", "pending", "approved", "rejected"
	SellerApplication *SellerApplication `json:"sellerApplication" bson:"sellerApplication"`
	VendorAccount     *VendorAccount     `json:"vendorAccount" bson:"vendorAccount"`
}

type UserPreferences struct {
	Categories        []string         `json:"categories,omitempty" bson:"categories,omitempty"`
	BudgetRange       string           `json:"budgetRange" bson:"budgetRange"`
	ShoppingFrequency string           `json:"shoppingFrequency" bson:"shoppingFrequency"`
	SpecialPrefs      *map[string]bool `json:"specialPrefs,omitempty" bson:"specialPrefs,omitempty"`
}

type UserInterests struct {
	Categories []string `json:"categories" bson:"categories" validate:"required,min=1,max=3,dive,required"`
	IsSet      bool     `json:"isSet" bson:"isSet"`
}

type UserProfile struct {
	Location       string `json:"location" bson:"location"`
	Bio            string `json:"bio" bson:"bio"`
	ProfilePicture string `json:"profileImage,omitempty" bson:"profileImage,omitempty"`
}

type SellerApplication struct {
	ID     primitive.ObjectID `bson:"_id,omitempty"`
	UserID primitive.ObjectID `bson:"userID"`

	// Tier Selection
	RequestedTier string `json:"requestedTier" bson:"requestedTier" validate:"required,oneof=individual verified business"`
	ApprovedTier  string `json:"approvedTier,omitempty" bson:"approvedTier,omitempty"`

	// Business Type Info
	BusinessTypeInfo *SellerBusinessInfo `json:"businessInfo" bson:"businessInfo"`
	IsRegistered     bool                `json:"isRegistered" bson:"isRegistered"`

	// Basic Info (All Tiers)
	StoreName        string   `json:"storeName" bson:"storeName" validate:"required,min=2,max=50"`
	StoreDescription string   `json:"storeDescription" bson:"storeDescription" validate:"required,min=50,max=500"`
	Categories       []string `json:"categories" bson:"categories" validate:"required,min=1,max=5"`

	// Business Details
	BusinessDetails *BusinessDetails `json:"businessDetails" bson:"businessDetails"`

	// Store Customization
	StoreDetails *StoreDetails `json:"storeDetails" bson:"storeDetails"`

	// Verification Documents
	IDDocument         *VerificationDocument  `json:"idDocument,omitempty" bson:"idDocument,omitempty"`
	SelfieVerification *VerificationDocument  `json:"selfieVerification,omitempty" bson:"selfieVerification,omitempty"`
	ProofOfActivity    []VerificationDocument `json:"proofOfActivity,omitempty" bson:"proofOfActivity,omitempty"`     // Tier 2+
	AddressProof       *VerificationDocument  `json:"addressProof,omitempty" bson:"addressProof,omitempty"`           // Tier 2+
	BusinessDocuments  []VerificationDocument `json:"businessDocuments,omitempty" bson:"businessDocuments,omitempty"` // Tier 3 only
	TaxID              string                 `json:"taxId,omitempty" bson:"taxId,omitempty"`                         // Tier 3 only
	SocialMedia        []SocialMediaLink      `json:"socialMedia,omitempty" bson:"socialMedia,omitempty"`

	// Terms & Verification
	TermsAccepted   bool      `json:"termsAccepted" bson:"termsAccepted" validate:"required,eq=true"`
	TermsAcceptedAt time.Time `json:"termsAcceptedAt" bson:"termsAcceptedAt"`

	// Application Status
	Status          string              `json:"status" bson:"status" validate:"required,oneof=draft pending under_review approved rejected"`
	RiskScore       int                 `json:"riskScore,omitempty" bson:"riskScore,omitempty"`
	RiskFlags       []string            `json:"riskFlags,omitempty" bson:"riskFlags,omitempty"`
	AppliedAt       time.Time           `json:"appliedAt" bson:"appliedAt"`
	ReviewedAt      *time.Time          `json:"reviewedAt,omitempty" bson:"reviewedAt,omitempty"`
	ReviewedBy      *primitive.ObjectID `json:"reviewedBy,omitempty" bson:"reviewedBy,omitempty"`
	ReviewNotes     string              `json:"reviewNotes,omitempty" bson:"reviewNotes,omitempty"`
	RejectionReason string              `json:"rejectionReason,omitempty" bson:"rejectionReason,omitempty"`

	// Metadata
	CreatedAt time.Time `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt" bson:"updatedAt"`
	Version   int       `json:"version" bson:"version"` // For optimistic locking
}

type VerificationDocument struct {
	DocumentType       string              `json:"documentType" bson:"documentType"` // "id", "selfie", "business_license", "tax_document", "bank_statement", "address_proof", etc.
	FileName           string              `json:"fileName" bson:"fileName"`
	FileURL            string              `json:"fileUrl" bson:"fileUrl"`                               // Cloudinary URL
	ThumbnailURL       string              `json:"thumbnailUrl,omitempty" bson:"thumbnailUrl,omitempty"` // Cloudinary thumbnail
	FileSize           int64               `json:"fileSize" bson:"fileSize"`                             // Size in bytes
	ContentType        string              `json:"contentType" bson:"contentType"`                       // "image/jpeg", "image/png", "application/pdf"
	UploadedAt         time.Time           `json:"uploadedAt" bson:"uploadedAt"`
	VerificationStatus string              `json:"verificationStatus" bson:"verificationStatus"` // "pending", "verified", "rejected"
	VerifiedAt         *time.Time          `json:"verifiedAt,omitempty" bson:"verifiedAt,omitempty"`
	VerifiedBy         *primitive.ObjectID `json:"verifiedBy,omitempty" bson:"verifiedBy,omitempty"` // Admin who verified
	RejectionReason    string              `json:"rejectionReason,omitempty" bson:"rejectionReason,omitempty"`
}
type SellerBusinessInfo struct {
	RequestedTier      string `json:"requestedTier" bson:"requestedTier" validate:"required,oneof=individual verified business"`
	BusinessType       string `json:"type" bson:"type" validate:"required,oneof=unregistered sole-proprietor partnership llc corporation nonprofit"`
	BusinessSize       string `json:"size" bson:"size" validate:"required,oneof=just_me 2-10 11-50 51-100 101+"`
	BusinessExperience string `json:"experience" bson:"experience" validate:"required,oneof=0-6months 6months-2years 2years-5years 5years-10years 10years+"`
}
type BusinessDetails struct {
	BusinessName string `json:"businessName,omitempty" bson:"businessName,omitempty"`
	Description  string `json:"description" bson:"description" validate:"required,min=50,max=1000"`
	Location     string `json:"location" bson:"location" validate:"required"`
	Url          string `json:"url,omitempty" bson:"url,omitempty" validate:"omitempty,url"`
}
type StoreDetails struct {
	StoreName        string `json:"storeName" bson:"storeName"`
	StoreDescription string `json:"storeDescription" bson:"storeDescription"`
	StoreLogo        string `json:"storeLogo,omitempty" bson:"storeLogo,omitempty"`
	PrimaryColor     string `json:"primaryColor,omitempty" bson:"primaryColor,omitempty"`
	AccentColor      string `json:"accentColor,omitempty" bson:"accentColor,omitempty"`
}

type VendorApplication struct {
	ID                  primitive.ObjectID `bson:"_id,omitempty"`
	UserID              primitive.ObjectID `bson:"userID"`
	BusinessName        string             `json:"businessName" bson:"businessName"`
	BusinessType        string             `json:"businessType" bson:"businessType"`
	BusinessDescription string             `json:"businessDescription" bson:"businessDescription"`
	ContactEmail        string             `json:"contactEmail" bson:"contactEmail"`
	ContactPhone        string             `json:"contactPhone" bson:"contactPhone"`
	BusinessAddress     string             `json:"businessAddress" bson:"businessAddress"`
	TaxID               string             `json:"taxId" bson:"taxId"`
	Website             string             `json:"website" bson:"website"`
	SocialMedia         []string           `json:"socialMedia" bson:"socialMedia"`
	Products            []string           `json:"products" bson:"products"`
	Experience          string             `json:"experience" bson:"experience"`
	Motivation          string             `json:"motivation" bson:"motivation"`
	Status              string             `json:"status" bson:"status"` // pending, approved, rejected
	AppliedAt           time.Time          `json:"appliedAt" bson:"appliedAt"`
	ReviewedAt          *time.Time         `json:"reviewedAt" bson:"reviewedAt"`
	ReviewedBy          string             `json:"reviewedBy" bson:"reviewedBy"`
	ReviewNotes         string             `json:"reviewNotes" bson:"reviewNotes"`
}
type VendorAccount struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"`
	UserID        primitive.ObjectID `bson:"userID"`
	ApplicationID primitive.ObjectID `bson:"applicationID"`
	CreatedAt     time.Time          `json:"createdAt" bson:"createdAt"`
	// Tier & Limits
	Tier           string     `json:"tier" bson:"tier"` // "individual", "verified", "business"
	TierUpgradedAt *time.Time `json:"tierUpgradedAt,omitempty" bson:"tierUpgradedAt,omitempty"`

	// Current Limits (based on tier)
	MaxProducts     int     `json:"maxProducts" bson:"maxProducts"`
	MaxMonthlySales float64 `json:"maxMonthlySales" bson:"maxMonthlySales"`
	TransactionFee  float64 `json:"transactionFee" bson:"transactionFee"` // Percentage
	PayoutHoldDays  int     `json:"payoutHoldDays" bson:"payoutHoldDays"`

	// Current Usage
	ProductCount      int     `json:"productCount" bson:"productCount"`
	CurrentMonthSales float64 `json:"currentMonthSales" bson:"currentMonthSales"`
	TotalSales        float64 `json:"totalSales" bson:"totalSales"`

	// Trust Score (builds over time)
	TrustScore      int `json:"trustScore" bson:"trustScore"` // 0-100
	TotalOrders     int `json:"totalOrders" bson:"totalOrders"`
	PositiveReviews int `json:"positiveReviews" bson:"positiveReviews"`
	DisputeCount    int `json:"disputeCount" bson:"disputeCount"`

	// Status
	Status     string `json:"status" bson:"status"` // "active", "suspended", "banned"
	IsVerified bool   `json:"isVerified" bson:"isVerified"`

	// Timestamps
	ActivatedAt time.Time  `json:"activatedAt" bson:"activatedAt"`
	LastSaleAt  *time.Time `json:"lastSaleAt,omitempty" bson:"lastSaleAt,omitempty"`
	UpdatedAt   time.Time  `json:"updatedAt" bson:"updatedAt"`
}
type UserOnboardingDraft struct {
	ID            primitive.ObjectID     `bson:"_id,omitempty"`
	UserID        primitive.ObjectID     `json:"userID" bson:"userID"`
	Role          string                 `json:"role" bson:"role"`
	Step          int                    `json:"step" bson:"step"`
	StepCompleted bool                   `json:"stepCompleted" bson:"stepCompleted"`
	StepData      map[string]interface{} `json:"stepData" bson:"stepData"`
	UpdatedAt     time.Time              `json:"updatedAt" bson:"updatedAt"`
	Version       int                    `json:"version" bson:"version"`
}
type SocialMediaLink struct {
	Platform string `json:"platform" bson:"platform"`
	Handle   string `json:"handle" bson:"handle"`
	URL      string `json:"url" bson:"url"`
	Verified bool   `json:"verified" bson:"verified"`
}

type RiskScore struct {
	Total     int      `json:"total"`
	Threshold int      `json:"threshold"`
	Flags     []string `json:"flags"`
}
type ApprovalDecision struct {
	Action string `json:"action"` // "AUTO_APPROVE", "MANUAL_REVIEW", "REJECT"
	Reason string `json:"reason"`
	Score  int    `json:"score"`
}
