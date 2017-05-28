package streaming

type order struct {
	ID     string  `json:"id"`
	Price  float64 `json:"price,string"`
	Volume float64 `json:"volume,string"`
}

type orderBook struct {
	Sequence int64    `json:"sequence,string"`
	Asks     []*order `json:"asks"`
	Bids     []*order `json:"bids"`
}

type tradeUpdate struct {
	Base    float64 `json:"base,string"`
	Counter float64 `json:"counter,string"`
	OrderID string  `json:"order_id"`
}

type createUpdate struct {
	OrderID string  `json:"order_id"`
	Type    string  `json:"type"`
	Price   float64 `json:"price,string"`
	Volume  float64 `json:"volume,string"`
}

type deleteUpdate struct {
	OrderID string `json:"order_id"`
}

type update struct {
	Sequence     int64          `json:"sequence,string"`
	TradeUpdates []*tradeUpdate `json:"trade_updates"`
	CreateUpdate *createUpdate  `json:"create_update"`
	DeleteUpdate *deleteUpdate  `json:"delete_update"`
	Timestamp    int64          `json:"timestamp"`
}

type credentials struct {
	APIKeyID     string `json:"api_key_id"`
	APIKeySecret string `json:"api_key_secret"`
}
