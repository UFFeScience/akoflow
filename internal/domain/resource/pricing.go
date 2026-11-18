package resource

import "math"

type UnitPrice struct {
	LinkID     string   `json:"linkId,omitempty"`
	Rate       *float64 `json:"rate"`
	Source     string   `json:"source,omitempty"`
	ObservedAt string   `json:"observedAt,omitempty"`
	Currency   string   `json:"currency,omitempty"`
}

func Price(rate float64, metadata map[string]any, knownKey string) UnitPrice {
	source, _ := metadata["priceSource"].(string)
	observedAt, _ := metadata["priceObservedAt"].(string)
	currency, _ := metadata["priceCurrency"].(string)
	known, specified := metadata[knownKey].(bool)
	result := UnitPrice{Source: source, ObservedAt: observedAt, Currency: currency}
	if !math.IsNaN(rate) && !math.IsInf(rate, 0) && rate >= 0 && (known || !specified && rate > 0) {
		result.Rate = &rate
	}
	return result
}

func (r Resource) ComputePrice() UnitPrice {
	return Price(r.PricePerSecond, r.Metadata, "pricePerSecondKnown")
}

func (r Resource) DiskPrice() UnitPrice {
	rate, _ := r.Metadata["diskPricePerGiBMonth"].(float64)
	return Price(rate, r.Metadata, "diskPricePerGiBMonthKnown")
}

func (l NetworkLink) TransferPrice() UnitPrice {
	price := Price(l.PricePerByte, l.Metadata, "pricePerByteKnown")
	price.LinkID = l.ID
	return price
}
