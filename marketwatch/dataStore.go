package marketwatch

import (
	"time"

	"github.com/antihax/goesi/esi"
)

// Order wrapper to find last touch time.
// Cast structure to market
type Order struct {
	Touched time.Time
	Order   esi.GetMarketsRegionIdOrders200Ok
}

// OrderChange Details of what changed on an order
type OrderChange struct {
	OrderID      int64
	LocationId   int64
	TypeID       int32
	VolumeChange int32
	VolumeRemain int32
	Price        float64
	Duration     int32
	IsBuyOrder   bool
	Changed      bool
}

// storeData returns changes or true if the item is new
func (s *MarketWatch) storeData(locationID int64, order Order) (OrderChange, bool) {
	change := OrderChange{
		OrderID:    order.Order.OrderId,
		LocationId: order.Order.LocationId,
		TypeID:     order.Order.TypeId,
		IsBuyOrder: order.Order.IsBuyOrder,
	}
	sMap, _ := s.market[locationID]
	v, loaded := sMap.LoadOrStore(order.Order.OrderId, order)
	if loaded {
		cOrder := v.(Order)
		if order.Order.VolumeRemain != cOrder.Order.VolumeRemain ||
			order.Order.Price != cOrder.Order.Price ||
			order.Order.Duration != cOrder.Order.Duration {
			change.Changed = true
			change.VolumeChange = cOrder.Order.VolumeRemain - order.Order.VolumeRemain
			change.VolumeRemain = order.Order.VolumeRemain
			change.Price = order.Order.Price
			change.Duration = order.Order.Duration
		}
		sMap.Store(order.Order.OrderId, order)
		return change, false
	} else {
		return change, true
	}
}

func (s *MarketWatch) expireOrders(locationID int64, t time.Time) []OrderChange {
	sMap, _ := s.market[locationID]
	changes := []OrderChange{}

	// Find any expired orders
	sMap.Range(
		func(k, v interface{}) bool {
			o := v.(Order)
			if t.After(o.Touched) {
				changes = append(changes, OrderChange{
					OrderID:      o.Order.OrderId,
					LocationId:   o.Order.LocationId,
					TypeID:       o.Order.TypeId,
					IsBuyOrder:   o.Order.IsBuyOrder,
					Changed:      true,
					VolumeChange: o.Order.VolumeRemain,
					VolumeRemain: 0,
					Price:        o.Order.Price,
					Duration:     o.Order.Duration,
				})
			}
			return true
		})

	// Delete them out of the map
	for _, c := range changes {
		sMap.Delete(c.OrderID)
	}

	return changes
}
