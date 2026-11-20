package application_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	. "github.com/dujiao-next/internal/modules/fulfillment/application"
	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	fulfillmentgormstore "github.com/dujiao-next/internal/modules/fulfillment/infrastructure/gormstore"
	ordercontract "github.com/dujiao-next/internal/modules/order/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	ordergormstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"
	procurementdomain "github.com/dujiao-next/internal/modules/procurement/domain"
	procurementgormstore "github.com/dujiao-next/internal/modules/procurement/infrastructure/gormstore"

	"github.com/dujiao-next/internal/constants"
	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"
	"github.com/dujiao-next/internal/platform/database/gormdb"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func setupFulfillmentServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:fulfillment_service_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&orderdomain.Order{},
		&orderdomain.OrderItem{},
		&fulfillmentdomain.Fulfillment{},
		&cardsecretdomain.Secret{},
		&cardsecretdomain.Batch{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	gormdb.DB = db
	return db
}

func TestCreateAutoFulfillmentRespectsSKUBoundary(t *testing.T) {
	db := setupFulfillmentServiceTestDB(t)
	now := time.Now()

	order := &orderdomain.Order{
		OrderNo:                 "FULFILL-SKU-001",
		UserID:                  1,
		Status:                  constants.OrderStatusPaid,
		Currency:                "CNY",
		OriginalAmount:          money.FromDecimal(decimal.NewFromInt(10)),
		DiscountAmount:          money.FromDecimal(decimal.Zero),
		PromotionDiscountAmount: money.FromDecimal(decimal.Zero),
		TotalAmount:             money.FromDecimal(decimal.NewFromInt(10)),
		WalletPaidAmount:        money.FromDecimal(decimal.Zero),
		OnlinePaidAmount:        money.FromDecimal(decimal.NewFromInt(10)),
		RefundedAmount:          money.FromDecimal(decimal.Zero),
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	orderItem := &orderdomain.OrderItem{
		OrderID:         order.ID,
		ProductID:       100,
		SKUID:           1001,
		TitleJSON:       jsonmap.JSON{"zh-CN": "测试商品"},
		UnitPrice:       money.FromDecimal(decimal.NewFromInt(10)),
		Quantity:        1,
		TotalPrice:      money.FromDecimal(decimal.NewFromInt(10)),
		FulfillmentType: constants.FulfillmentTypeAuto,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(orderItem).Error; err != nil {
		t.Fatalf("create order item failed: %v", err)
	}

	secretTarget := &cardsecretdomain.Secret{
		ProductID: 100,
		SKUID:     1001,
		Secret:    "SECRET-SKU-1001",
		Status:    cardsecretdomain.StatusAvailable,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Create(secretTarget).Error; err != nil {
		t.Fatalf("create target secret failed: %v", err)
	}
	secretOther := &cardsecretdomain.Secret{
		ProductID: 100,
		SKUID:     1002,
		Secret:    "SECRET-SKU-1002",
		Status:    cardsecretdomain.StatusAvailable,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Create(secretOther).Error; err != nil {
		t.Fatalf("create other secret failed: %v", err)
	}

	svc := New(Options{
		OrderStore:       ordergormstore.New(db, "test-guest-credential-secret-with-32-bytes"),
		FulfillmentStore: fulfillmentgormstore.New(db),
	})

	result, err := svc.CreateAuto(order.ID)
	if err != nil {
		t.Fatalf("create auto fulfillment failed: %v", err)
	}
	if result == nil {
		t.Fatalf("fulfillment should not be nil")
	}
	if !strings.Contains(result.Payload, "SECRET-SKU-1001") {
		t.Fatalf("payload should contain target sku secret, got: %s", result.Payload)
	}
	if strings.Contains(result.Payload, "SECRET-SKU-1002") {
		t.Fatalf("payload should not contain other sku secret, got: %s", result.Payload)
	}

	var targetAfter cardsecretdomain.Secret
	if err := db.First(&targetAfter, secretTarget.ID).Error; err != nil {
		t.Fatalf("query target secret failed: %v", err)
	}
	if targetAfter.Status != cardsecretdomain.StatusUsed {
		t.Fatalf("target secret status want used got %s", targetAfter.Status)
	}

	var otherAfter cardsecretdomain.Secret
	if err := db.First(&otherAfter, secretOther.ID).Error; err != nil {
		t.Fatalf("query other secret failed: %v", err)
	}
	if otherAfter.Status != cardsecretdomain.StatusAvailable {
		t.Fatalf("other secret status should stay available got %s", otherAfter.Status)
	}

	var orderAfter orderdomain.Order
	if err := db.First(&orderAfter, order.ID).Error; err != nil {
		t.Fatalf("query order failed: %v", err)
	}
	if orderAfter.Status != constants.OrderStatusCompleted {
		t.Fatalf("order status want completed got %s", orderAfter.Status)
	}
}

// refundBeforeTxStore 在发货事务开始前把订单改成已全额退款，
// 模拟「事务外状态检查通过之后、发货事务之前退款先提交」的并发时序。
type refundBeforeTxStore struct {
	ordercontract.Store
	db      *gorm.DB
	orderID uint
}

func (s refundBeforeTxStore) WithinTransaction(fn func(ordercontract.Transaction) error) error {
	if err := s.db.Model(&orderdomain.Order{}).Where("id = ?", s.orderID).
		Update("status", constants.OrderStatusRefunded).Error; err != nil {
		return err
	}
	return s.Store.WithinTransaction(fn)
}

func TestFulfillmentRejectedWhenRefundCommitsBeforeDeliveryTransaction(t *testing.T) {
	for _, mode := range []string{"auto", "manual"} {
		t.Run(mode, func(t *testing.T) {
			db := setupFulfillmentServiceTestDB(t)
			now := time.Now()
			ten := money.FromDecimal(decimal.NewFromInt(10))
			order := &orderdomain.Order{
				OrderNo: "FULFILL-RACE-" + mode, UserID: 1, Status: constants.OrderStatusPaid, Currency: "CNY",
				OriginalAmount: ten, TotalAmount: ten, OnlinePaidAmount: ten, CreatedAt: now, UpdatedAt: now,
			}
			if err := db.Create(order).Error; err != nil {
				t.Fatalf("create order failed: %v", err)
			}
			if err := db.Create(&orderdomain.OrderItem{
				OrderID: order.ID, ProductID: 100, SKUID: 1001, TitleJSON: jsonmap.JSON{"zh-CN": "测试商品"},
				UnitPrice: ten, Quantity: 1, TotalPrice: ten, FulfillmentType: mode, CreatedAt: now, UpdatedAt: now,
			}).Error; err != nil {
				t.Fatalf("create order item failed: %v", err)
			}
			secret := &cardsecretdomain.Secret{
				ProductID: 100, SKUID: 1001, Secret: "SECRET-RACE", Status: cardsecretdomain.StatusAvailable,
				CreatedAt: now, UpdatedAt: now,
			}
			if err := db.Create(secret).Error; err != nil {
				t.Fatalf("create secret failed: %v", err)
			}

			svc := New(Options{
				OrderStore: refundBeforeTxStore{
					Store: ordergormstore.New(db, "test-guest-credential-secret-with-32-bytes"), db: db, orderID: order.ID,
				},
				FulfillmentStore: fulfillmentgormstore.New(db),
			})

			var err error
			if mode == "auto" {
				_, err = svc.CreateAuto(order.ID)
			} else {
				_, err = svc.CreateManual(CreateManualInput{OrderID: order.ID, AdminID: 1, Payload: "manual content"})
			}
			if !errors.Is(err, ErrOrderStatusInvalid) {
				t.Fatalf("want ErrOrderStatusInvalid, got %v", err)
			}

			var after orderdomain.Order
			if err := db.First(&after, order.ID).Error; err != nil {
				t.Fatalf("query order failed: %v", err)
			}
			if after.Status != constants.OrderStatusRefunded {
				t.Fatalf("order status want refunded got %s", after.Status)
			}
			var count int64
			if err := db.Model(&fulfillmentdomain.Fulfillment{}).Where("order_id = ?", order.ID).Count(&count).Error; err != nil {
				t.Fatalf("count fulfillments failed: %v", err)
			}
			if count != 0 {
				t.Fatalf("expected no fulfillment, got %d", count)
			}
			var secretAfter cardsecretdomain.Secret
			if err := db.First(&secretAfter, secret.ID).Error; err != nil {
				t.Fatalf("query secret failed: %v", err)
			}
			if secretAfter.Status != cardsecretdomain.StatusAvailable {
				t.Fatalf("secret status want available got %s", secretAfter.Status)
			}
		})
	}
}

func TestManualFallbackStopsRejectedProcurement(t *testing.T) {
	for _, tc := range []struct {
		name            string
		status          string
		upstreamOrderID uint
		leaseActive     bool
		confirmed       bool
		success         bool
	}{
		{name: "confirmed rejection", status: constants.ProcurementStatusRejected, confirmed: true, success: true},
		{name: "missing confirmation", status: constants.ProcurementStatusRejected},
		{name: "retry scheduled", status: constants.ProcurementStatusFailed, confirmed: true},
		{name: "supplier order exists", status: constants.ProcurementStatusRejected, confirmed: true, upstreamOrderID: 1},
		{name: "worker still active", status: constants.ProcurementStatusRejected, confirmed: true, leaseActive: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupFulfillmentServiceTestDB(t)
			sqlDB, err := db.DB()
			if err != nil {
				t.Fatal(err)
			}
			sqlDB.SetMaxOpenConns(1)
			t.Cleanup(func() { _ = sqlDB.Close() })
			if err := db.AutoMigrate(&procurementdomain.Order{}); err != nil {
				t.Fatal(err)
			}
			parent := orderdomain.Order{OrderNo: "MANUAL-FALLBACK", Status: constants.OrderStatusPaid, Currency: "CNY"}
			if err := db.Create(&parent).Error; err != nil {
				t.Fatal(err)
			}
			order := orderdomain.Order{OrderNo: "MANUAL-FALLBACK-01", ParentID: &parent.ID, Status: constants.OrderStatusPaid, Currency: "CNY"}
			if err := db.Create(&order).Error; err != nil {
				t.Fatal(err)
			}
			item := orderdomain.OrderItem{OrderID: order.ID, ProductID: 1, SKUID: 1, Quantity: 1, TitleJSON: jsonmap.JSON{"en-US": "Test product"}, FulfillmentType: constants.FulfillmentTypeUpstream}
			if err := db.Create(&item).Error; err != nil {
				t.Fatal(err)
			}
			purchase := procurementdomain.Order{
				ConnectionID: 1, LocalOrderID: order.ID, LocalOrderNo: order.OrderNo,
				Status: tc.status, UpstreamOrderID: tc.upstreamOrderID, Currency: "CNY",
			}
			if tc.leaseActive {
				until := time.Now().UTC().Add(time.Minute)
				purchase.SubmitLeaseUntil = &until
			}
			procurements := procurementgormstore.New(db)
			if err := procurements.Create(&purchase); err != nil {
				t.Fatal(err)
			}
			svc := New(Options{
				OrderStore:       ordergormstore.New(db, "test-guest-credential-secret-with-32-bytes"),
				FulfillmentStore: fulfillmentgormstore.New(db),
			})
			result, err := svc.CreateManual(CreateManualInput{
				OrderID: order.ID, AdminID: 1, Payload: "Delivered separately",
				UpstreamNotChargedConfirmed: tc.confirmed,
			})
			wantPurchase, wantOrder := tc.status, constants.OrderStatusPaid
			if tc.success {
				if err != nil || result == nil || result.Status != constants.FulfillmentStatusDelivered {
					t.Fatalf("expected successful manual delivery, got %v, %v", result, err)
				}
				wantPurchase, wantOrder = constants.ProcurementStatusCanceled, constants.OrderStatusDelivered
				updated, err := procurements.UpdateStatus(purchase.ID, constants.ProcurementStatusRejected, constants.ProcurementStatusPending, nil)
				if err != nil || updated {
					t.Fatalf("canceled procurement must reject retry, updated=%v, error=%v", updated, err)
				}
			} else if !errors.Is(err, ErrProcurementFallbackUnavailable) || result != nil {
				t.Fatalf("expected the fallback to be blocked, got %v, %v", result, err)
			}
			if err := db.First(&purchase, purchase.ID).Error; err != nil || purchase.Status != wantPurchase {
				t.Fatalf("want procurement %s, got %s, %v", wantPurchase, purchase.Status, err)
			}
			if err := db.First(&order, order.ID).Error; err != nil || order.Status != wantOrder {
				t.Fatalf("want order %s, got %s, %v", wantOrder, order.Status, err)
			}
			if err := db.First(&parent, parent.ID).Error; err != nil || parent.Status != wantOrder {
				t.Fatalf("want parent order %s, got %s, %v", wantOrder, parent.Status, err)
			}
		})
	}
}
