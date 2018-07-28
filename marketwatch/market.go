package marketwatch

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/antihax/goesi/esi"
	"github.com/antihax/goesi/optional"
)

func (s *MarketWatch) startUpMarketWorkers() {
	// Get all the regions and fire up workers for each
	regions, _, err := s.esi.ESI.UniverseApi.GetUniverseRegions(context.Background(), nil)
	if err != nil {
		log.Fatal(err)
	}

	for _, region := range regions {
		// Prebuild the maps
		s.market[int64(region)] = &sync.Map{}

		if region < 11000000 || region == 11000031 {
			go s.marketWorker(region)
		}
	}
}

func (s *MarketWatch) marketWorker(regionID int32) {
	// For totalization
	wg := sync.WaitGroup{}

	// Loop forever
	for {
		start := time.Now()
		numOrders := 0

		// Return Channels
		rchan := make(chan []esi.GetMarketsRegionIdOrders200Ok, 100000)
		echan := make(chan error, 100000)

		orders, res, err := s.esi.ESI.MarketApi.GetMarketsRegionIdOrders(
			context.Background(), "all", regionID, nil,
		)
		if err != nil {
			log.Println(err)
			continue
		}
		rchan <- orders

		// Figure out if there are more pages
		pages, err := getPages(res)
		if err != nil {
			log.Println(err)
			continue
		}

		// Get the other pages concurrently
		for pages > 1 {
			wg.Add(1) // count whats running
			go func(page int32) {
				defer wg.Done() // release when done

				orders, _, err := s.esi.ESI.MarketApi.GetMarketsRegionIdOrders(
					context.Background(),
					"all",
					regionID,
					&esi.GetMarketsRegionIdOrdersOpts{Page: optional.NewInt32(page)},
				)

				if err != nil {
					echan <- err
					return
				}

				// Add the orders to the channel
				rchan <- orders
			}(pages)
			pages--
		}

		wg.Wait() // Wait for everything to finish

		// Close the channels
		close(rchan)
		close(echan)

		for err := range echan {
			// Start over if any requests failed
			log.Println(err)
			continue
		}

		changes := []OrderChange{}
		newOrders := []esi.GetMarketsRegionIdOrders200Ok{}
		// Add all the orders together
		for o := range rchan {
			for i := range o {
				change, isNew := s.storeData(int64(regionID), Order{Touched: start, Order: o[i]})
				numOrders++
				if change.Changed && !isNew {
					changes = append(changes, change)
				}
				if isNew {
					newOrders = append(newOrders, o[i])
				}
			}
		}
		deletions := s.expireOrders(int64(regionID), start)

		if len(newOrders) > 0 {
			s.broadcast.Broadcast(
				Message{
					Action:  "addition",
					Payload: newOrders,
				},
			)
		}

		if len(changes) > 0 {
			s.broadcast.Broadcast(
				Message{
					Action:  "change",
					Payload: changes,
				},
			)
		}

		if len(deletions) > 0 {
			s.broadcast.Broadcast(
				Message{
					Action:  "deletion",
					Payload: deletions,
				},
			)
		}

		duration := timeUntilCacheExpires(res)
		if duration < time.Second { // Sleep at least a minute
			duration = time.Second * 10
		}

		// Sleep until the cache timer expires, plus a little.
		time.Sleep(duration + time.Second*15)
	}
}
