package bitx

import "time"
import "net/http"
import "net/url"
import "errors"
import "encoding/json"
import "strconv"
import "fmt"

const userAgent = "bitx-go/0.0.1"
var base = url.URL{Scheme: "https", Host: "bitx.co.za"}

type Client struct {
}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) get(path string, params url.Values, result interface{}) error {
	u := base
	u.Path = path
	u.RawQuery = params.Encode()
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Add("User-Agent", userAgent)
	r, err := (&http.Client{}).Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		return errors.New(fmt.Sprintf(
			"BitX error %d: %s", r.StatusCode, r.Status))
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
}

type Ticker struct {
	Timestamp time.Time
	Bid, Ask, Last float64
}

func (c *Client) Ticker(pair string) (Ticker, error) {
	var r ticker
	err := c.get("/api/1/ticker", url.Values{"pair": {pair}}, &r)
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

	return Ticker{t, bid, ask, last}, nil
}


type orderbookEntry struct {
	Price string `json:"price"`
	Volume string `json:"volume"`
}

type orderbook struct {
	Error string `json:"error"`
	Asks []orderbookEntry `json:"asks"`
	Bids []orderbookEntry `json:"bids"`
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

func (c *Client) OrderBook(pair string) (
	bids, asks []OrderBookEntry, err error) {

	var r orderbook
	err = c.get("/api/1/orderbook", url.Values{"pair": {pair}}, &r)
	if err != nil {
		return nil, nil, err
	}
	if r.Error != "" {
		return nil, nil, errors.New("BitX error: " + r.Error)
	}

	return convert(r.Bids), convert(r.Asks), nil
}
