package marketwatch

import (
	"log"

	"github.com/antihax/goesi/esi"
)

// Message wraps different payloads for the websocket interface
type Message struct {
	Action  string      `json:"action"`
	Payload interface{} `json:"payload"`
}

func (s *MarketWatch) dumpMarket() interface{} {
	m := []esi.GetMarketsRegionIdOrders200Ok{}
	log.Printf("Dumping market\n")
	// loop all the locations and dump into the structure
	for _, r := range s.market {
		r.Range(
			func(k, v interface{}) bool {
				o := v.(Order)
				m = append(m, o.Order)
				return true
			})
	}
	log.Printf("dump complete\n")
	// return the whole thing.
	return Message{
		Action:  "addition",
		Payload: m,
	}
}
