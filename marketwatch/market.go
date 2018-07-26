package marketwatch

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/antihax/goesi"

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
	log.Printf("start regionID %d\n", regionID)
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

		// Add all the orders together
		for o := range rchan {
			for i := range o {
				change := s.storeData(int64(regionID), Order{Touched: start, Order: o[i]})
				numOrders++
				if change.Changed {
					fmt.Printf("%+v\n", change)
				}
			}
		}

		changes := s.expireOrders(int64(regionID), start)
		for _, change := range changes {
			fmt.Printf("%+v\n", change)
		}
		duration := timeUntilCacheExpires(res)
		if duration < time.Second { // Sleep at least a minute
			fmt.Printf("weird stuff happened R %d # %d %s %s\n", regionID, len(orders), time.Now().UTC().String(), goesi.CacheExpires(res).UTC().String())
			duration = time.Second * 10
		}

		sanic := time.Since(start)
		log.Printf("completed regionID %d with %d orders in %s sleeping for %s \n", regionID, numOrders, sanic.String(), duration.String())

		// Sleep until the cache timer expires, plus a little.
		time.Sleep(duration + time.Second*15)
	}
}
