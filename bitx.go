// Go wrapper for the BitX API.
// The API is documented here: https://bitx.co/api
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

var base = url.URL{Scheme: "https", Host: "api.mybitx.com"}

type Client struct {
	api_key_id, api_key_secret string
}

// Pass an empty string for the api_key_id if you will only access the public
// API.
func NewClient(api_key_id, api_key_secret string) *Client {
	return &Client{api_key_id, api_key_secret}
}

func (c *Client) call(method, path string, params url.Values,
	result interface{}) error {
	u := base
	u.Path = path

	var body *bytes.Reader
	if method == "GET" {
		u.RawQuery = params.Encode()
		body = bytes.NewReader(nil)
	} else if method == "POST" {
		body = bytes.NewReader([]byte(params.Encode()))
	} else {
		return errors.New("Unsupported method")
	}

	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return err
	}
	if c.api_key_id != "" {
		req.SetBasicAuth(c.api_key_id, c.api_key_secret)
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

	if err := json.NewDecoder(r.Body).Decode(result); err != nil {
		return err
	}

	return nil
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
func (c *Client) PostOrder(pair string, order_type OrderType,
	volume, price float64) (string, error) {
	form := make(url.Values)
	form.Add("volume", fmt.Sprintf("%f", volume))
	form.Add("price", fmt.Sprintf("%f", price))
	form.Add("pair", pair)
	form.Add("type", string(order_type))

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

// Returns a list of the most recently placed orders.
// The list is truncated after 100 items.
func (c *Client) ListOrders(pair string) ([]Order, error) {
	var r orders
	err := c.call("GET", "/api/1/listorders", url.Values{"pair": {pair}}, &r)
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
	Asset       string
	Balance     float64
	Reserved    float64
	Unconfirmed float64
}

func parseBalances(bal []balance) []Balance {
	var bl []Balance
	for _, b := range bal {
		var r Balance
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

func (c *Client) Send(amount, currency, address, desc, message string) error {
	form := make(url.Values)
	form.Add("amount", amount)
	form.Add("currency", currency)
	form.Add("address", address)
	form.Add("description", desc)
	form.Add("message", message)

	var r stoporder
	err := c.call("POST", "/api/1/send", form, &r)
	if err != nil {
		return err
	}
	if r.Error != "" {
		return errors.New("BitX error: " + r.Error)
	}

	return nil
}
