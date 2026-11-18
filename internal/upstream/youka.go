package upstream

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/shopspring/decimal"
)

const youkaAPI = "/api/v1/open"

var youkaOrderRef = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,64}$`)
var youkaReason = regexp.MustCompile(`^[A-Z0-9_]{1,64}$`)

type YoukaAdapter struct {
	origin string
	key    string
	secret []byte
	client *http.Client
	images *DujiaoNextAdapter
}

func newYoukaAdapter(conn *siteconnectiondomain.Connection, uploadsDir string) (*YoukaAdapter, error) {
	base, err := url.Parse(conn.BaseURL)
	if err != nil || base.Scheme != "https" || base.Hostname() == "" || base.User != nil ||
		(base.Path != "" && base.Path != "/") || base.RawQuery != "" || base.ForceQuery || base.Fragment != "" {
		return nil, errors.New("Youka base URL must be the HTTPS team origin")
	}
	secret, err := hex.DecodeString(strings.TrimSpace(conn.ApiSecret))
	key := strings.TrimSpace(conn.ApiKey)
	if err != nil || len(secret) != 32 || !strings.HasPrefix(key, "ak_") || len(key) == 3 {
		return nil, errors.New("invalid Youka API key or secret")
	}
	return &YoukaAdapter{
		origin: "https://" + base.Host, key: key, secret: secret,
		client: &http.Client{Timeout: 25 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}},
		images: NewDujiaoNextAdapter(conn, uploadsDir),
	}, nil
}

func (a *YoukaAdapter) request(ctx context.Context, method, endpoint string, query url.Values, body []byte, out any) error {
	apiPath := youkaAPI + endpoint
	rawQuery := query.Encode()
	target := a.origin + apiPath
	if rawQuery != "" {
		target += "?" + rawQuery
	}
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return errors.New("build Youka request")
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return err
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := hex.EncodeToString(random)
	digest := sha256.Sum256(body)
	message := strings.Join([]string{method, apiPath, rawQuery, timestamp, nonce, hex.EncodeToString(digest[:])}, "\n")
	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(message))
	req.Header.Set("X-Api-Key", a.key)
	req.Header.Set("X-Api-Timestamp", timestamp)
	req.Header.Set("X-Api-Nonce", nonce)
	req.Header.Set("X-Api-Signature", hex.EncodeToString(mac.Sum(nil)))
	req.Header.Set("User-Agent", "GPT-Suchong/1.0")
	if len(body) != 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("Youka request failed: %w", err)
	}
	defer resp.Body.Close()
	const maxResponse = 8 << 20
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil || len(raw) > maxResponse {
		return errors.New("invalid Youka response size")
	}
	var envelope struct {
		Code   *int            `json:"code"`
		Reason string          `json:"reason"`
		Data   json.RawMessage `json:"data"`
	}
	decodeErr := json.Unmarshal(raw, &envelope)
	if resp.StatusCode != http.StatusOK || (envelope.Code != nil && *envelope.Code != 0) {
		reason := envelope.Reason
		if !youkaReason.MatchString(reason) {
			reason = ""
		}
		return &upstreamHTTPError{Status: resp.StatusCode, Code: reason}
	}
	if decodeErr != nil || envelope.Code == nil || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return errors.New("invalid Youka response")
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return errors.New("invalid Youka response data")
	}
	return nil
}

func (a *YoukaAdapter) Ping(ctx context.Context) (*PingResult, error) {
	var ping struct {
		Scopes []string `json:"scopes"`
		IP     string   `json:"ip"`
	}
	if err := a.request(ctx, http.MethodGet, "/ping", nil, nil, &ping); err != nil {
		return nil, err
	}
	result := &PingResult{
		SiteName: "Youka", ProtocolVersion: "open/v1", Currency: "CNY",
		MemberLevel: map[string]interface{}{"scopes": ping.Scopes, "ip": ping.IP},
	}
	for _, scope := range ping.Scopes {
		if scope != "wallet:read" {
			continue
		}
		var wallet struct{ Balance decimal.Decimal }
		if err := a.request(ctx, http.MethodGet, "/wallet", nil, nil, &wallet); err != nil {
			return nil, err
		}
		result.Balance = wallet.Balance.String()
		break
	}
	return result, nil
}

func (*YoukaAdapter) ListCategories(context.Context) (*CategoryListResult, error) {
	return &CategoryListResult{Categories: []UpstreamCategory{}}, nil
}

func (a *YoukaAdapter) catalog(ctx context.Context) ([]UpstreamProduct, error) {
	var rows []struct {
		SKUID        uint            `json:"skuId"`
		ProductID    uint            `json:"productId"`
		ProductName  string          `json:"productName"`
		SKUName      string          `json:"skuName"`
		UnitPrice    decimal.Decimal `json:"unitPrice"`
		ReadyStock   *int            `json:"readyStock"`
		ConfirmIntro bool            `json:"confirmIntro"`
		Currency     string          `json:"currency"`
	}
	if err := a.request(ctx, http.MethodGet, "/products", nil, nil, &rows); err != nil {
		return nil, err
	}
	products := map[uint]*UpstreamProduct{}
	for _, row := range rows {
		if row.ConfirmIntro || row.SKUID == 0 || row.ProductID == 0 || row.Currency != "CNY" {
			continue
		}
		product := products[row.ProductID]
		price := row.UnitPrice.String()
		if product == nil {
			product = &UpstreamProduct{
				ID: row.ProductID, Slug: fmt.Sprintf("youka-%d", row.ProductID),
				Title: jsonmap.JSON{"zh-CN": row.ProductName}, PriceAmount: price,
				Currency: "CNY", FulfillmentType: constants.FulfillmentTypeUpstream, IsActive: true,
			}
			products[row.ProductID] = product
		}
		if minPrice, _ := decimal.NewFromString(product.PriceAmount); row.UnitPrice.LessThan(minPrice) {
			product.PriceAmount = price
		}
		quantity, availability := 0, "out_of_stock"
		if row.ReadyStock != nil && *row.ReadyStock > 0 {
			quantity, availability = *row.ReadyStock, "in_stock"
		}
		product.SKUs = append(product.SKUs, UpstreamSKU{
			ID: row.SKUID, SKUCode: strconv.FormatUint(uint64(row.SKUID), 10), PriceAmount: price,
			SpecValues:  jsonmap.JSON{"zh-CN": row.SKUName},
			StockStatus: availability, StockQuantity: quantity, IsActive: true,
		})
	}
	result := make([]UpstreamProduct, 0, len(products))
	for _, product := range products {
		sort.Slice(product.SKUs, func(i, j int) bool { return product.SKUs[i].ID < product.SKUs[j].ID })
		result = append(result, *product)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (a *YoukaAdapter) ListProducts(ctx context.Context, opts ListProductsOpts) (*ProductListResult, error) {
	products, err := a.catalog(ctx)
	if err != nil {
		return nil, err
	}
	page, size := max(1, opts.Page), opts.PageSize
	if size <= 0 {
		size = 20
	}
	result := &ProductListResult{Total: len(products), Items: []UpstreamProduct{}}
	if len(products) == 0 || page-1 > (len(products)-1)/size {
		return result, nil
	}
	start := (page - 1) * size
	result.Items = products[start : start+min(size, len(products)-start)]
	return result, nil
}

func (a *YoukaAdapter) GetProduct(ctx context.Context, productID uint) (*UpstreamProduct, error) {
	products, err := a.catalog(ctx)
	if err != nil {
		return nil, err
	}
	for _, product := range products {
		if product.ID == productID {
			return &product, nil
		}
	}
	return nil, ErrUpstreamProductUnavailable
}

func (a *YoukaAdapter) DownloadImage(ctx context.Context, imageURL string) (string, error) {
	return a.images.DownloadImage(ctx, imageURL)
}

type youkaOrderRequest struct {
	ClientOrderID  string      `json:"clientOrderId"`
	SKUID          uint        `json:"skuId"`
	Quantity       int         `json:"qty"`
	ExpectedAmount json.Number `json:"expectedAmount"`
}

type youkaPreparedOrder struct {
	Connection string          `json:"connection"`
	Body       json.RawMessage `json:"body"`
}

func (a *YoukaAdapter) connectionFingerprint() string {
	digest := sha256.Sum256([]byte(a.origin + "\n" + a.key))
	return hex.EncodeToString(digest[:])
}

type youkaOrder struct {
	OrderNo       string          `json:"orderNo"`
	ClientOrderID string          `json:"clientOrderId"`
	Status        string          `json:"status"`
	SKUID         uint            `json:"skuId"`
	OrderedQty    int             `json:"orderedQty"`
	Qty           int             `json:"qty"`
	Amount        decimal.Decimal `json:"amount"`
	Refunded      decimal.Decimal `json:"refundedAmount"`
	Cards         []struct {
		ID          string `json:"cardId"`
		Secret      string `json:"secret"`
		UsageMethod string `json:"usageMethod"`
		Voided      bool   `json:"voided"`
	} `json:"cards"`
	Tutorials []struct {
		Text string `json:"text"`
	} `json:"tutorials"`
}

func (a *YoukaAdapter) PrepareOrder(ctx context.Context, ref string, skuID uint, quantity int, maxAmount, currency string) (string, error) {
	cap, err := decimal.NewFromString(maxAmount)
	if err != nil || currency != "CNY" || !youkaOrderRef.MatchString(ref) || ref == "." || ref == ".." || skuID == 0 || quantity <= 0 {
		return "", errors.New("invalid Youka purchase inputs; the storefront order must use CNY")
	}
	var quote struct {
		Amount       *decimal.Decimal `json:"amount"`
		Stock        *int             `json:"stock"`
		ConfirmIntro bool             `json:"confirmIntro"`
	}
	query := url.Values{"skuId": {strconv.FormatUint(uint64(skuID), 10)}, "qty": {strconv.Itoa(quantity)}}
	if err := a.request(ctx, http.MethodGet, "/quote", query, nil, &quote); err != nil {
		return "", err
	}
	if quote.ConfirmIntro {
		return "", errors.New("Youka SKU requires buyer confirmation of product terms")
	}
	if quote.Amount == nil || quote.Amount.IsNegative() || quote.Amount.GreaterThan(cap) {
		return "", errors.New("Youka purchase quote exceeds the local order amount or is invalid")
	}
	if quote.Stock == nil || *quote.Stock < quantity {
		return "", errors.New("Youka stock is insufficient")
	}
	body, err := json.Marshal(youkaOrderRequest{ref, skuID, quantity, json.Number(quote.Amount.String())})
	if err != nil {
		return "", err
	}
	snapshot, err := json.Marshal(youkaPreparedOrder{Connection: a.connectionFingerprint(), Body: body})
	return string(snapshot), err
}

func (a *YoukaAdapter) CreateOrder(ctx context.Context, req CreateUpstreamOrderReq) (*CreateUpstreamOrderResp, error) {
	var prepared youkaPreparedOrder
	if json.Unmarshal([]byte(req.PreparedOrder), &prepared) != nil || len(prepared.Body) == 0 {
		return nil, errors.New("missing or invalid frozen Youka order request")
	}
	if prepared.Connection != a.connectionFingerprint() {
		return &CreateUpstreamOrderResp{
			ErrorCode: "CLIENT_ORDER_CONFLICT", ErrorMessage: "Youka connection changed after the purchase request was saved",
		}, nil
	}
	var snapshot youkaOrderRequest
	if json.Unmarshal(prepared.Body, &snapshot) != nil || snapshot.ClientOrderID != req.DownstreamOrderNo ||
		!youkaOrderRef.MatchString(snapshot.ClientOrderID) || snapshot.ClientOrderID == "." || snapshot.ClientOrderID == ".." ||
		snapshot.SKUID == 0 || snapshot.Quantity <= 0 || snapshot.ExpectedAmount == "" {
		return nil, errors.New("missing or invalid frozen Youka order request")
	}
	order, err := a.lookupOrder(ctx, snapshot.ClientOrderID)
	// An ambiguous submit is reconciled by the original client ID before another POST.
	var upstreamErr *upstreamHTTPError
	if errors.As(err, &upstreamErr) && upstreamErr.Status == http.StatusNotFound && upstreamErr.Code == "NOT_FOUND" {
		err = a.request(ctx, http.MethodPost, "/orders", nil, prepared.Body, &order)
	}
	if err != nil {
		if errors.As(err, &upstreamErr) && upstreamErr.Status < 500 && upstreamErr.Status != http.StatusTooManyRequests && upstreamErr.Code != "" {
			return &CreateUpstreamOrderResp{ErrorCode: upstreamErr.Code, ErrorMessage: "Youka: " + upstreamErr.Code}, nil
		}
		return nil, err
	}
	quoted, _ := decimal.NewFromString(snapshot.ExpectedAmount.String())
	if order.ClientOrderID != snapshot.ClientOrderID || order.SKUID != snapshot.SKUID ||
		order.quantity() != snapshot.Quantity || !order.Amount.Equal(quoted) {
		return &CreateUpstreamOrderResp{ErrorCode: "CLIENT_ORDER_CONFLICT", ErrorMessage: "Youka order does not match the saved purchase request"}, nil
	}
	detail, err := order.detail()
	if err != nil {
		return nil, err
	}
	return &CreateUpstreamOrderResp{
		OK: true, OrderNo: detail.OrderNo, Status: detail.Status,
		Amount: detail.Amount, Currency: detail.Currency, Fulfillment: detail.Fulfillment,
	}, nil
}

func (a *YoukaAdapter) lookupOrder(ctx context.Context, ref string) (youkaOrder, error) {
	var order youkaOrder
	if !youkaOrderRef.MatchString(ref) || ref == "." || ref == ".." {
		return order, errors.New("invalid Youka order reference")
	}
	err := a.request(ctx, http.MethodGet, "/orders/"+url.PathEscape(ref), nil, nil, &order)
	return order, err
}

func (a *YoukaAdapter) GetOrder(ctx context.Context, _ uint, orderNo ...string) (*UpstreamOrderDetail, error) {
	if len(orderNo) == 0 {
		return nil, errors.New("Youka order reference is required")
	}
	order, err := a.lookupOrder(ctx, orderNo[0])
	if err != nil {
		return nil, err
	}
	if orderNo[0] != order.OrderNo && orderNo[0] != order.ClientOrderID {
		return nil, errors.New("Youka returned the wrong order")
	}
	return order.detail()
}

func (*YoukaAdapter) CancelOrder(context.Context, uint) error {
	return ErrUpstreamCancelNotAllowed
}

func (o youkaOrder) quantity() int {
	if o.OrderedQty > 0 {
		return o.OrderedQty
	}
	return o.Qty
}

func (o youkaOrder) detail() (*UpstreamOrderDetail, error) {
	if !youkaOrderRef.MatchString(o.OrderNo) || o.quantity() <= 0 {
		return nil, errors.New("missing Youka order number or quantity")
	}
	detail := &UpstreamOrderDetail{OrderNo: o.OrderNo, Amount: o.Amount.String(), Currency: "CNY", RefundedAmount: o.Refunded.String()}
	codes := make([]string, 0, len(o.Cards))
	ids := make(map[string]bool, len(o.Cards))
	for _, card := range o.Cards {
		if card.Voided || card.ID == "" || strings.TrimSpace(card.Secret) == "" || ids[card.ID] {
			continue
		}
		ids[card.ID] = true
		content := card.Secret
		if usage := strings.TrimSpace(card.UsageMethod); usage != "" {
			content += "\n" + usage
		}
		codes = append(codes, content)
	}
	if len(codes) > o.quantity() {
		return nil, errors.New("Youka returned more cards than ordered")
	}
	switch o.Status {
	case "STOCKOUT":
		detail.Status = "accepted"
		if len(codes) > 0 {
			detail.Status = constants.OrderStatusPartiallyDelivered
		}
	case "REFUNDED":
		detail.Status = "refunded"
	case "DELIVERED":
		if len(codes) == o.quantity() && o.Refunded.IsZero() {
			detail.Status = "delivered"
		} else if o.Refunded.IsPositive() && len(codes) < o.quantity() {
			detail.Status = "partially_refunded"
		} else {
			return nil, errors.New("Youka delivery count does not match the order")
		}
	default:
		return nil, errors.New("unknown Youka order status")
	}
	if len(codes) > 0 && detail.Status != "refunded" {
		count := len(codes)
		for _, tutorial := range o.Tutorials {
			if text := strings.TrimSpace(tutorial.Text); text != "" {
				codes = append(codes, text)
			}
		}
		status := constants.OrderStatusPartiallyDelivered
		if detail.Status == "delivered" {
			status = "delivered"
		}
		detail.Fulfillment = &UpstreamFulfillment{
			Type: constants.FulfillmentTypeUpstream, Status: status,
			Payload: strings.Join(codes, "\n\n"), DeliveryData: jsonmap.JSON{"supplier_card_count": count},
		}
	}
	return detail, nil
}
