package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/developia-II/ecommerce-backend/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type TransactionRepository interface {
	GetBalance(ctx context.Context, vendorID primitive.ObjectID) (models.VendorAccount, error)
	GetTransactions(ctx context.Context, vendorID primitive.ObjectID, limit int) ([]models.Transaction, error)
	GetPayouts(ctx context.Context, vendorID primitive.ObjectID) ([]models.PayoutRequest, error)
	RequestPayout(ctx context.Context, payout models.PayoutRequest) error
	CreditVendorForSale(ctx context.Context, vendorID primitive.ObjectID, amount float64, orderID primitive.ObjectID, orderNumber string) error
	MaturateFunds(ctx context.Context, vendorID primitive.ObjectID) error
}

type MongoTransactionRepository struct {
	DB *mongo.Database
}

func NewTransactionRepository(db *mongo.Database) TransactionRepository {
	return &MongoTransactionRepository{DB: db}
}

func (r *MongoTransactionRepository) GetBalance(ctx context.Context, vendorID primitive.ObjectID) (models.VendorAccount, error) {
	var account models.VendorAccount
	collection := r.DB.Collection("vendorAccounts")
	err := collection.FindOne(ctx, bson.M{"userID": vendorID}).Decode(&account)
	return account, err
}

func (r *MongoTransactionRepository) GetTransactions(ctx context.Context, vendorID primitive.ObjectID, limit int) ([]models.Transaction, error) {
	collection := r.DB.Collection("transactions")

	opts := options.Find().SetSort(bson.M{"createdAt": -1})
	if limit > 0 {
		opts.SetLimit(int64(limit))
	}

	cursor, err := collection.Find(ctx, bson.M{"vendorId": vendorID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var transactions []models.Transaction
	if err := cursor.All(ctx, &transactions); err != nil {
		return nil, err
	}
	return transactions, nil
}

func (r *MongoTransactionRepository) GetPayouts(ctx context.Context, vendorID primitive.ObjectID) ([]models.PayoutRequest, error) {
	collection := r.DB.Collection("payouts")

	opts := options.Find().SetSort(bson.M{"requestedAt": -1})
	cursor, err := collection.Find(ctx, bson.M{"vendorId": vendorID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var payouts []models.PayoutRequest
	if err := cursor.All(ctx, &payouts); err != nil {
		return nil, err
	}
	return payouts, nil
}

func (r *MongoTransactionRepository) RequestPayout(ctx context.Context, payout models.PayoutRequest) error {
	payoutColl := r.DB.Collection("payouts")
	accountColl := r.DB.Collection("vendorAccounts")

	// Use Transaction for atomicity if supported, but simple check and update for now
	// 1. Double check available balance
	var account models.VendorAccount
	err := accountColl.FindOne(ctx, bson.M{"userID": payout.VendorID}).Decode(&account)
	if err != nil {
		return err
	}

	if account.AvailableBalance < payout.Amount {
		return fmt.Errorf("insufficient available balance")
	}

	// 2. Insert payout request
	_, err = payoutColl.InsertOne(ctx, payout)
	if err != nil {
		return err
	}

	// 3. Deduct from available balance (move to "withdrawn" or just subtract)
	// We'll just subtract it. If it fails, we have a consistency issue, but this is a start.
	_, err = accountColl.UpdateOne(ctx,
		bson.M{"userID": payout.VendorID},
		bson.M{"$inc": bson.M{"availableBalance": -payout.Amount}},
	)

	return err
}

func (r *MongoTransactionRepository) CreditVendorForSale(ctx context.Context, vendorID primitive.ObjectID, amount float64, orderID primitive.ObjectID, orderNumber string) error {
	accountColl := r.DB.Collection("vendorAccounts")
	txColl := r.DB.Collection("transactions")

	// 1. Get vendor account to know tier and fees
	var account models.VendorAccount
	err := accountColl.FindOne(ctx, bson.M{"userID": vendorID}).Decode(&account)
	if err != nil {
		return fmt.Errorf("vendor account not found for processing sale: %v", err)
	}

	// 2. Calculate fee and net
	fee := amount * (account.TransactionFee / 100)
	netAmount := amount - fee

	// 3. Create Transaction Record
	holdDuration := time.Duration(account.PayoutHoldDays) * 24 * time.Hour
	holdUntil := time.Now().Add(holdDuration)

	transaction := models.Transaction{
		ID:        primitive.NewObjectID(),
		VendorID:  vendorID,
		OrderID:   &orderID,
		Type:      models.TransactionTypeSale,
		Status:    models.TransactionStatusPending,
		Amount:    netAmount,
		Fee:       fee,
		Currency:  "USD", // Default
		Reference: orderNumber,
		HoldUntil: &holdUntil,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err = txColl.InsertOne(ctx, transaction)
	if err != nil {
		return err
	}

	// 4. Update Vendor Balance
	update := bson.M{
		"$inc": bson.M{
			"pendingBalance":    netAmount,
			"lifeTimeEarnings":  netAmount,
			"currentMonthSales": amount, // Track gross for tier limits
			"totalSales":        amount,
		},
		"$set": bson.M{"updatedAt": time.Now()},
	}

	_, err = accountColl.UpdateOne(ctx, bson.M{"userID": vendorID}, update)
	return err
}

func (r *MongoTransactionRepository) MaturateFunds(ctx context.Context, vendorID primitive.ObjectID) error {
	accountColl := r.DB.Collection("vendorAccounts")
	txColl := r.DB.Collection("transactions")

	// 1. Find all pending transactions for this vendor where HoldUntil has passed
	filter := bson.M{
		"vendorId":  vendorID,
		"status":    models.TransactionStatusPending,
		"holdUntil": bson.M{"$lte": time.Now()},
	}

	cursor, err := txColl.Find(ctx, filter)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	var maturedTxs []models.Transaction
	if err := cursor.All(ctx, &maturedTxs); err != nil {
		return err
	}

	if len(maturedTxs) == 0 {
		return nil // Nothing to maturate
	}

	var totalToMaturate float64
	var txIDs []primitive.ObjectID
	for _, tx := range maturedTxs {
		totalToMaturate += tx.Amount
		txIDs = append(txIDs, tx.ID)
	}

	// 2. Update Vendor Account
	// Subtract from pending, add to available
	_, err = accountColl.UpdateOne(ctx,
		bson.M{"userID": vendorID},
		bson.M{
			"$inc": bson.M{
				"pendingBalance":   -totalToMaturate,
				"availableBalance": totalToMaturate,
			},
			"$set": bson.M{"updatedAt": time.Now()},
		},
	)
	if err != nil {
		return err
	}

	// 3. Mark transactions as available
	_, err = txColl.UpdateMany(ctx,
		bson.M{"_id": bson.M{"$in": txIDs}},
		bson.M{
			"$set": bson.M{
				"status":    models.TransactionStatusAvailable,
				"updatedAt": time.Now(),
			},
		},
	)

	return err
}
