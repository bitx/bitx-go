// Go wrapper for the Luno API.
// The API is documented here: https://www.luno.com/api
package bitx

import (
	"bytes"
	_ "crypto/sha512"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"
)

const userAgent = "bitx-go/0.0.3"

var defaultBaseURL = url.URL{Scheme: "https", Host: "api.mybitx.com"}

type Client struct {
	apiKeyID, apiKeySecret string
	baseURL                url.URL
}

// Pass an empty string for the api_key_id if you will only access the public
// API.
func NewClient(apiKeyID, apiKeySecret string) *Client {
	return &Client{apiKeyID, apiKeySecret, defaultBaseURL}
}

type errorResp struct {
	Error     string `json:"error"`
	ErrorCode string `json:"error_code"`
}

func (c *Client) call(method, path string, params url.Values,
	result interface{}) error {
	u := c.baseURL
	u.Path = path

	var body *bytes.Reader
	if method == "GET" {
		u.RawQuery = params.Encode()
		body = bytes.NewReader(nil)
	} else if method == "POST" || method == "PUT" || method == "PATCH" {
		body = bytes.NewReader([]byte(params.Encode()))
	} else if method == "DELETE" {
		body = bytes.NewReader(nil)
	} else {
		return errors.New("Unsupported method")
	}

	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return err
	}
	if c.apiKeyID != "" {
		req.SetBasicAuth(c.apiKeyID, c.apiKeySecret)
	}
	req.Header.Add("User-Agent", userAgent)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	r, err := (&http.Client{}).Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(r.Body)
		return errors.New(fmt.Sprintf(
			"BitX error %d: %s: %s",
			r.StatusCode, r.Status, string(body)))
	}

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	var errResult errorResp
	if err := json.Unmarshal(data, &errResult); err != nil {
		return err
	}

	if errResult.Error != "" || errResult.ErrorCode != "" {
		return fmt.Errorf("bitx: remote error %s %s", errResult.ErrorCode, errResult.Error)
	}

	return json.Unmarshal(data, &result)
}

type ticker struct {
	Error     string `json:"error"`
	Timestamp int64  `json:"timestamp"`
	Bid       string `json:"bid"`
	Ask       string `json:"ask"`
	Last      string `json:"last_trade"`
	Volume24H string `json:"rolling_24_hour_volume"`
}

type Ticker struct {
	Timestamp                 time.Time
	Bid, Ask, Last, Volume24H float64
}

// Returns the latest ticker indicators for the given currency pair..
func (c *Client) Ticker(pair string) (Ticker, error) {
	var r ticker
	err := c.call("GET", "/api/1/ticker", url.Values{"pair": {pair}}, &r)
	if err != nil {
		return Ticker{}, err
	}
	if r.Error != "" {
		return Ticker{}, errors.New("BitX error: " + r.Error)
	}

	t := time.Unix(r.Timestamp/1000, 0)

	bid, err := strconv.ParseFloat(r.Bid, 64)
	if err != nil {
		return Ticker{}, err
	}

	ask, err := strconv.ParseFloat(r.Ask, 64)
	if err != nil {
		return Ticker{}, err
	}

	last, err := strconv.ParseFloat(r.Last, 64)
	if err != nil {
		return Ticker{}, err
	}

	volume24h, err := strconv.ParseFloat(r.Volume24H, 64)
	if err != nil {
		return Ticker{}, err
	}

	return Ticker{t, bid, ask, last, volume24h}, nil
}

type orderbookEntry struct {
	Price  string `json:"price"`
	Volume string `json:"volume"`
}

type orderbook struct {
	Error string           `json:"error"`
	Asks  []orderbookEntry `json:"asks"`
	Bids  []orderbookEntry `json:"bids"`
}

type OrderBookEntry struct {
	Price, Volume float64
}

func convert(entries []orderbookEntry) (r []OrderBookEntry) {
	r = make([]OrderBookEntry, len(entries))
	for i, e := range entries {
		price, _ := strconv.ParseFloat(e.Price, 64)
		volume, _ := strconv.ParseFloat(e.Volume, 64)
		r[i].Price = price
		r[i].Volume = volume
	}
	return r
}

// Returns a list of bids and asks in the order book for the given currency
// pair.
func (c *Client) OrderBook(pair string) (
	bids, asks []OrderBookEntry, err error) {

	var r orderbook
	err = c.call("GET", "/api/1/orderbook", url.Values{"pair": {pair}}, &r)
	if err != nil {
		return nil, nil, err
	}
	if r.Error != "" {
		return nil, nil, errors.New("BitX error: " + r.Error)
	}

	return convert(r.Bids), convert(r.Asks), nil
}

type trade struct {
	Timestamp int64  `json:"timestamp"`
	Price     string `json:"price"`
	Volume    string `json:"volume"`
}

type trades struct {
	Error  string  `json:"error"`
	Trades []trade `json:"trades"`
}

type Trade struct {
	Timestamp     time.Time
	Price, Volume float64
}

// Returns a list of the most recent trades for the given currency pair.
func (c *Client) Trades(pair string) ([]Trade, error) {
	var r trades
	err := c.call("GET", "/api/1/trades", url.Values{"pair": {pair}}, &r)
	if err != nil {
		return nil, err
	}
	if r.Error != "" {
		return nil, errors.New("BitX error: " + r.Error)
	}

	tr := make([]Trade, len(r.Trades))
	for i, t := range r.Trades {
		tr[i].Timestamp = time.Unix(t.Timestamp/1000, 0)
		price, _ := strconv.ParseFloat(t.Price, 64)
		volume, _ := strconv.ParseFloat(t.Volume, 64)
		tr[i].Price = price
		tr[i].Volume = volume
	}
	return tr, nil
}

type postorder struct {
	OrderId string `json:"order_id"`
	Error   string `json:"error"`
}

type OrderType string

const BID = OrderType("BID")
const ASK = OrderType("ASK")

// Create a new trade order.
// Specify zero for baseAccountID and counterAccountID to use your default
// accounts.
func (c *Client) PostOrder(pair string, order_type OrderType,
	volume, price float64,
	baseAccountID, counterAccountID string) (string, error) {
	form := make(url.Values)
	form.Add("volume", fmt.Sprintf("%f", volume))
	form.Add("price", fmt.Sprintf("%f", price))
	form.Add("pair", pair)
	form.Add("type", string(order_type))
	if baseAccountID != "" {
		form.Add("base_account_id", baseAccountID)
	}
	if counterAccountID != "" {
		form.Add("counter_account_id", counterAccountID)
	}

	var r postorder
	err := c.call("POST", "/api/1/postorder", form, &r)
	if err != nil {
		return "", err
	}
	if r.Error != "" {
		return "", errors.New("BitX error: " + r.Error)
	}

	return r.OrderId, nil
}

type order struct {
	Error             string `json:"error"`
	OrderId           string `json:"order_id"`
	CreationTimestamp int64  `json:"creation_timestamp"`
	Type              string `json:"type"`
	State             string `json:"state"`
	LimitPrice        string `json:"limit_price"`
	LimitVolume       string `json:"limit_volume"`
	Base              string `json:"base"`
	Counter           string `json:"counter"`
	FeeBase           string `json:"fee_base"`
	FeeCounter        string `json:"fee_counter"`
}

type orders struct {
	Error  string  `json:"error"`
	Orders []order `json:"orders"`
}

type OrderState string

const Pending = OrderState("PENDING")
const Complete = OrderState("COMPLETE")

type Order struct {
	Id                  string
	CreatedAt           time.Time
	Type                OrderType
	State               OrderState
	LimitPrice          float64
	LimitVolume         float64
	Base, Counter       float64
	FeeBase, FeeCounter float64
}

func atofloat64(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func parseOrder(bo order) Order {
	var o Order
	o.Id = bo.OrderId
	o.Type = OrderType(bo.Type)
	o.State = OrderState(bo.State)
	o.CreatedAt = time.Unix(bo.CreationTimestamp/1000, 0)
	o.LimitPrice = atofloat64(bo.LimitPrice)
	o.LimitVolume = atofloat64(bo.LimitVolume)
	o.Base = atofloat64(bo.Base)
	o.Counter = atofloat64(bo.Counter)
	o.FeeBase = atofloat64(bo.FeeBase)
	o.FeeCounter = atofloat64(bo.FeeCounter)
	return o
}

// Returns a list of placed orders.
// The list is truncated after 100 items.
// If state is an empty string, the list won't be filtered by state.
func (c *Client) ListOrders(pair string, state OrderState) ([]Order, error) {
	params := url.Values{"pair": {pair}}
	if state != "" {
		params.Add("state", string(state))
	}

	var r orders
	err := c.call("GET", "/api/1/listorders", params, &r)
	if err != nil {
		return nil, err
	}
	if r.Error != "" {
		return nil, errors.New("BitX error: " + r.Error)
	}

	orders := make([]Order, len(r.Orders))
	for i, bo := range r.Orders {
		orders[i] = parseOrder(bo)
	}
	return orders, nil
}

var pathIDRegex = regexp.MustCompile("^[[:alnum:]]+$")

func isValidPathID(id string) bool {
	if len(id) == 0 || len(id) > 255 {
		return false
	}
	return pathIDRegex.MatchString(id)
}

// Get an order by its id.
func (c *Client) GetOrder(id string) (*Order, error) {
	if !isValidPathID(id) {
		return nil, errors.New("invalid order id")
	}
	var bo order
	err := c.call("GET", "/api/1/orders/"+id, nil, &bo)
	if err != nil {
		return nil, err
	}
	if bo.Error != "" {
		return nil, errors.New("BitX error: " + bo.Error)
	}
	o := parseOrder(bo)
	return &o, nil
}

type stoporder struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

// Request to stop an order.
func (c *Client) StopOrder(id string) error {
	form := make(url.Values)
	form.Add("order_id", id)
	var r stoporder
	err := c.call("POST", "/api/1/stoporder", form, &r)
	if err != nil {
		return err
	}
	if r.Error != "" {
		return errors.New("BitX error: " + r.Error)
	}
	return nil
}

type balance struct {
	AccountID   string `json:"account_id"`
	Asset       string `json:"asset"`
	Balance     string `json:"balance"`
	Reserved    string `json:"reserved"`
	Unconfirmed string `json:"unconfirmed"`
}

type balances struct {
	Error   string    `json:"error"`
	Balance []balance `json:"balance"`
}

type Balance struct {
	AccountID   string `json:"account_id"`
	Asset       string
	Balance     float64
	Reserved    float64
	Unconfirmed float64
}

func parseBalances(bal []balance) []Balance {
	var bl []Balance
	for _, b := range bal {
		var r Balance
		r.AccountID = b.AccountID
		r.Asset = b.Asset
		r.Balance = atofloat64(b.Balance)
		r.Reserved = atofloat64(b.Reserved)
		r.Unconfirmed = atofloat64(b.Unconfirmed)
		bl = append(bl, r)
	}
	return bl
}

// Returns the trading account balance and reserved funds.
func (c *Client) Balance(asset string) (
	balance float64, reserved float64, err error) {
	var r balances
	err = c.call("GET", "/api/1/balance", url.Values{"asset": {asset}}, &r)
	if err != nil {
		return 0, 0, err
	}
	if r.Error != "" {
		return 0, 0, errors.New("BitX error: " + r.Error)
	}
	if len(r.Balance) == 0 {
		return 0, 0, errors.New("Balance not returned")
	}
	bl := parseBalances(r.Balance)
	return bl[0].Balance, bl[0].Reserved, nil
}

// Balances return the balances of all accounts.
func (c *Client) Balances() ([]Balance, error) {
	var r balances
	err := c.call("GET", "/api/1/balance", nil, &r)
	if err != nil {
		return nil, err
	}
	if r.Error != "" {
		return nil, errors.New("BitX error: " + r.Error)
	}
	return parseBalances(r.Balance), nil
}

type sendResp struct {
	Success      bool   `json:"success"`
	WithdrawalID string `json:"withdrawal_id"`
}

func (c *Client) Send(amount, currency, address, desc, message string) (string, error) {
	form := make(url.Values)
	form.Add("amount", amount)
	form.Add("currency", currency)
	form.Add("address", address)
	form.Add("description", desc)
	form.Add("message", message)

	var r sendResp
	err := c.call("POST", "/api/1/send", form, &r)

	return r.WithdrawalID, err
}

type address struct {
	Asset            string `json:"asset"`
	Address          string `json:"address"`
	TotalReceived    string `json:"total_received"`
	TotalUnconfirmed string `json:"total_unconfirmed"`
	Error            string `json:"error"`
}

type Address struct {
	Asset            string
	Address          string
	TotalReceived    float64
	TotalUnconfirmed float64
}

func parseAddress(a address) (Address, error) {
	if a.Error != "" {
		return Address{}, errors.New("BitX error: " + a.Error)
	}
	var r Address
	r.Asset = a.Asset
	r.Address = a.Address
	r.TotalReceived = atofloat64(a.TotalReceived)
	r.TotalUnconfirmed = atofloat64(a.TotalUnconfirmed)

	return r, nil
}

// GetReceiveAddress returns the default receive address associated with your
// account and the amount received via the address, but can take optional
// parameter to check non-default address
func (c *Client) GetReceiveAddress(asset string, receiveAddress string) (Address, error) {
	var a address
	urlValues := url.Values{"asset": {asset}, "address": {receiveAddress}}
	err := c.call("GET", "/api/1/funding_address", urlValues, &a)
	if err != nil {
		return Address{}, err
	}

	return parseAddress(a)
}

// NewReceiveAddress allocates a new receive address to your account.
// There is a rate limit of 1 address per hour, but bursts of up to 10
// addresses are allowed.
func (c *Client) NewReceiveAddress(asset string) (Address, error) {
	var a address
	urlValues := url.Values{"asset": {asset}}
	err := c.call("POST", "/api/1/funding_address", urlValues, &a)
	if err != nil {
		return Address{}, err
	}

	return parseAddress(a)
}

// FeeInfo hold information about the user's fees and trading volume.
type FeeInfo struct {
	ThirtyDayVolume float64 `json:"thirty_day_volume,string"`
	MakerFee        float64 `json:"maker_fee,string"`
	TakerFee        float64 `json:"taker_fee,string"`
}

// GetFeeInfo returns information about the user's fees and trading volume.
func (c *Client) GetFeeInfo(pair string) (FeeInfo, error) {
	var fi FeeInfo
	urlValues := url.Values{"pair": {pair}}
	err := c.call("GET", "/api/1/fee_info", urlValues, &fi)
	if err != nil {
		return FeeInfo{}, err
	}

	return fi, nil
}

// QuoteResponse contains information about a specific quote
type QuoteResponse struct {
	ID            int64   `json:"id,string"`
	Type          string  `json:"type"`
	Pair          string  `json:"pair"`
	BaseAmount    float64 `json:"base_amount,string"`
	CounterAmount float64 `json:"counter_amount,string"`
	CreatedAt     int64   `json:"created_at"`
	ExpiresAt     int64   `json:"expires_at"`
	Discarded     bool    `json:"discarded"`
	Exercised     bool    `json:"exercised"`
}

// CreateQuote creates a quote of the given type (BUY or SELL) for the given
// baseAmount of a specific pair (like XBTZAR)
func (c *Client) CreateQuote(quoteType, baseAmount, pair string) (QuoteResponse, error) {
	if quoteType != "BUY" && quoteType != "SELL" {
		return QuoteResponse{}, errors.New("quoteType must be either 'BUY' or 'SELL'")
	}
	var qr QuoteResponse
	urlValues := url.Values{"type": {quoteType}, "base_amount": {baseAmount}, "pair": {pair}}
	err := c.call("POST", "/api/1/quotes", urlValues, &qr)
	if err != nil {
		return QuoteResponse{}, err
	}

	return qr, nil
}

func (c *Client) quoteHandler(id, method string) (QuoteResponse, error) {
	var qr QuoteResponse
	err := c.call(method, "/api/1/quotes/"+id, nil, &qr)

	if err != nil {
		return QuoteResponse{}, err
	}

	return qr, nil
}

// GetQuote returns the details of the specified quote
func (c *Client) GetQuote(id string) (QuoteResponse, error) {
	return c.quoteHandler(id, "GET")
}

// ExerciseQuote accepts the given quote
func (c *Client) ExerciseQuote(id string) (QuoteResponse, error) {
	return c.quoteHandler(id, "PUT")
}

// DeleteQuote rejects a quote
func (c *Client) DeleteQuote(id string) (QuoteResponse, error) {
	return c.quoteHandler(id, "DELETE")
}

type OrderTrade struct {
	Base       float64   `json:"base,string"`
	Counter    float64   `json:"counter,string"`
	FeeBase    float64   `json:"fee_base,string"`
	FeeCounter float64   `json:"fee_counter,string"`
	IsBuy      bool      `json:"is_buy"`
	OrderID    string    `json:"order_id"`
	Pair       string    `json:"pair"`
	Price      float64   `json:"price,string"`
	Timestamp  int64     `json:"timestamp"`
	Type       OrderType `json:"type"`
	Volume     float64   `json:"volume,string"`
}

type tradeResp struct {
	Trades []OrderTrade `json:"trades"`
}

// ListTrades returns trades in your account for the given pair, sortest by
// oldest first, since the given timestamp.
func (c *Client) ListTrades(pair string, since int64) ([]OrderTrade, error) {
	params := url.Values{
		"pair":  {pair},
		"since": {strconv.FormatInt(since, 10)},
	}
	var resp tradeResp
	err := c.call("GET", "/api/1/listtrades", params, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Trades, nil
}

type Withdrawal struct {
	ID        string  `json:"id"`
	Status    string  `json:"status"`
	CreatedAt int64   `json:"created_at"`
	Type      string  `json:"type"`
	Currency  string  `json:"currency"`
	Amount    float64 `json:"amount,string"`
	Fee       float64 `json:"fee,string"`
}

func (c *Client) GetWithdrawal(id string) (*Withdrawal, error) {
	var w Withdrawal
	err := c.call("GET", "/api/1/withdrawals/"+id, nil, &w)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

type WithdrawalList struct {
	Withdrawals []Withdrawal `json:"withdrawals"`
}

func (c *Client) GetWithdrawals() (*WithdrawalList, error) {
	var w WithdrawalList
	err := c.call("GET", "/api/1/withdrawals", nil, &w)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

// For internal use.
func (c *Client) SetBaseURL(url url.URL) {
	c.baseURL = url
}
