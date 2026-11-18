package upstream

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/dujiao-next/internal/constants"
	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
)

func TestYoukaRecoversChargedOrderAndDeliversByReference(t *testing.T) {
	secret := strings.Repeat("ab", 32)
	nonces := map[string]bool{}
	charged := false
	submitCount := 0
	ready := false
	var mu sync.Mutex
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		body, _ := io.ReadAll(req.Body)
		if req.Header.Get("X-Api-Key") != "ak_test" || req.UserAgent() != "GPT-Suchong/1.0" {
			t.Errorf("missing client headers")
		}
		nonce := req.Header.Get("X-Api-Nonce")
		if len(nonce) != 32 || nonces[nonce] {
			t.Errorf("missing or repeated nonce")
		}
		nonces[nonce] = true
		hash := sha256.Sum256(body)
		key, _ := hex.DecodeString(secret)
		mac := hmac.New(sha256.New, key)
		mac.Write([]byte(strings.Join([]string{req.Method, req.URL.EscapedPath(), req.URL.RawQuery,
			req.Header.Get("X-Api-Timestamp"), nonce, hex.EncodeToString(hash[:])}, "\n")))
		if !hmac.Equal([]byte(req.Header.Get("X-Api-Signature")), []byte(hex.EncodeToString(mac.Sum(nil)))) {
			t.Errorf("signature mismatch")
		}

		switch req.Method + " " + req.URL.Path {
		case "GET /api/v1/open/quote":
			if req.URL.Query().Get("skuId") == "10" {
				io.WriteString(w, `{"code":0,"data":{"amount":18.03,"stock":2,"confirmIntro":true}}`)
				return
			}
			if req.URL.Query().Get("skuId") != "9" || req.URL.Query().Get("qty") != "2" {
				t.Errorf("incorrect quote query")
			}
			io.WriteString(w, `{"code":0,"data":{"amount":18.03,"stock":2}}`)
		case "GET /api/v1/open/orders/DJ-1", "GET /api/v1/open/orders/AGT-1":
			if !charged {
				w.WriteHeader(http.StatusNotFound)
				io.WriteString(w, `{"code":404,"reason":"NOT_FOUND"}`)
				return
			}
			if ready {
				io.WriteString(w, `{"code":0,"data":{"orderNo":"AGT-1","clientOrderId":"DJ-1","status":"DELIVERED","skuId":9,"orderedQty":2,"amount":18.03,"cards":[{"cardId":"c1","secret":"card-A"},{"cardId":"c1","secret":"duplicate"},{"cardId":"c2","secret":"card-B"},{"cardId":"void","secret":"voided","voided":true}]}}`)
			} else {
				io.WriteString(w, `{"code":0,"data":{"orderNo":"AGT-1","clientOrderId":"DJ-1","status":"STOCKOUT","skuId":9,"orderedQty":2,"amount":18.03,"cards":[{"cardId":"c1","secret":"card-A"}]}}`)
			}
		case "POST /api/v1/open/orders":
			submitCount++
			var order youkaOrderRequest
			if json.Unmarshal(body, &order) != nil || order.ExpectedAmount != "18.03" || order.ClientOrderID != "DJ-1" {
				t.Errorf("submit did not preserve quote and order ID")
			}
			charged = true
			w.WriteHeader(http.StatusBadGateway)
		default:
			t.Errorf("unexpected request %s %s", req.Method, req.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	adapter, err := newYoukaAdapter(&siteconnectiondomain.Connection{BaseURL: server.URL, ApiKey: "ak_test", ApiSecret: secret}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	adapter.client = server.Client()
	if _, err := adapter.PrepareOrder(t.Context(), "DJ-1", 9, 2, "18.02", "CNY"); err == nil {
		t.Fatal("a quote above the local amount was accepted")
	}
	if _, err := adapter.PrepareOrder(t.Context(), "DJ-1", 10, 2, "20.00", "CNY"); err == nil {
		t.Fatal("a quote requiring buyer confirmation was accepted")
	}
	snapshot, err := adapter.PrepareOrder(t.Context(), "DJ-1", 9, 2, "20.00", "CNY")
	if err != nil {
		t.Fatal(err)
	}
	req := CreateUpstreamOrderReq{DownstreamOrderNo: "DJ-1", PreparedOrder: snapshot}
	otherConnection := *adapter
	otherConnection.key = "ak_other"
	if rejected, err := otherConnection.CreateOrder(t.Context(), req); err != nil || rejected.OK || rejected.ErrorCode != "CLIENT_ORDER_CONFLICT" {
		t.Fatal("a prepared request was allowed to move to another account")
	}
	if _, err := adapter.CreateOrder(t.Context(), req); err == nil {
		t.Fatal("an uncertain transport result was reported as success")
	}
	recovered, err := adapter.CreateOrder(t.Context(), req)
	mu.Lock()
	posts := submitCount
	ready = true
	mu.Unlock()
	if err != nil || !recovered.OK || recovered.OrderNo != "AGT-1" || recovered.Status != constants.OrderStatusPartiallyDelivered ||
		recovered.Fulfillment == nil || recovered.Fulfillment.Payload != "card-A" || posts != 1 {
		t.Fatalf("unexpected recovery result, error=%v, POSTs=%d", err, posts)
	}
	order, err := adapter.GetOrder(t.Context(), 0, "AGT-1")
	if err != nil || order.Status != "delivered" || order.Fulfillment == nil || order.Fulfillment.Payload != "card-A\n\ncard-B" {
		t.Fatalf("unexpected final delivery, error=%v", err)
	}
}
