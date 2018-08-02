package marketwatch

import (
	"github.com/antihax/goesi/esi"
)

// Message wraps different payloads for the websocket interface
type Message struct {
	Action  string      `json:"action"`
	Payload interface{} `json:"payload"`
}

func (s *MarketWatch) dumpMarket(send chan interface{}) {

	// Prevent changes to the map while we loop
	s.mmutex.RLock()
	defer s.mmutex.RUnlock()

	// loop all the locations
	for _, r := range s.market {
		// Build a list
		m := []esi.GetMarketsRegionIdOrders200Ok{}
		r.Range(
			func(k, v interface{}) bool {
				o := v.(Order)
				m = append(m, o.Order)
				return true
			})
		// send the list out
		if len(m) > 0 {
			send <- Message{
				Action:  "addition",
				Payload: m,
			}
		}
	}
}
