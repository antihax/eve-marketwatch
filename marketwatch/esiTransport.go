package marketwatch

import (
	"log"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var apiTransportLimiter chan bool
var urlFilterRe *regexp.Regexp

func init() {
	// concurrency limiter
	// 100 concurrent requests should fill 1 connection
	apiTransportLimiter = make(chan bool, 100)
	urlFilterRe = regexp.MustCompile("/v[0-9]{1}/|/[0-9]+/")
}

// ApiTransport custom transport to chain into the HTTPClient to gather statistics.
type ApiTransport struct {
	next *http.Transport
}

// RoundTrip wraps http.DefaultTransport.RoundTrip to provide stats and handle error rates.
func (t *ApiTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Limit concurrency
	apiTransportLimiter <- true

	// Free the worker
	defer func() { <-apiTransportLimiter }() // Loop until success

	tries := 0
	for {
		// Tickup retry counter
		tries++

		// Run the request and time the response
		start := time.Now()
		res, triperr := t.next.RoundTrip(req)
		end := time.Now()

		endpoint := urlFilterRe.ReplaceAllString(req.URL.Path, "/")

		// We got a response
		if res != nil {
			// Log metrics
			metricAPICalls.With(
				prometheus.Labels{
					"host":     req.Host,
					"endpoint": endpoint,
					"status":   strconv.Itoa(res.StatusCode),
					"try":      strconv.Itoa(tries),
				},
			).Observe(float64(end.Sub(start).Nanoseconds()) / float64(time.Millisecond))

			// Get the ESI error information
			resetS := res.Header.Get("x-esi-error-limit-reset")
			tokensS := res.Header.Get("x-esi-error-limit-remain")

			// If we cannot decode this is likely from another source.
			esiRateLimiter := true
			reset, err := strconv.ParseFloat(resetS, 64)
			if err != nil {
				esiRateLimiter = false
			}
			tokens, err := strconv.ParseFloat(tokensS, 64)
			if err != nil {
				esiRateLimiter = false
			}

			// Tick up and log any errors
			if res.StatusCode >= 400 {
				metricAPIErrors.Inc()
				log.Printf("St: %d Res: %s Tok: %s - %s\n", res.StatusCode, resetS, tokensS, req.URL)
				if !esiRateLimiter { // Not an ESI error
					time.Sleep(time.Second * time.Duration(tries))
				}
			}

			// Backoff
			if res.StatusCode == 420 { // Something went wrong
				time.Sleep(time.Duration(reset) * time.Second)
			} else if esiRateLimiter { // Sleep based on error rate.
				percentRemain := 1 - (tokens / 100)
				duration := reset * percentRemain
				time.Sleep(time.Second * time.Duration(duration))
			}

			// Get out for "our bad" statuses
			if res.StatusCode >= 400 && res.StatusCode < 420 {
				if res.StatusCode != 403 {
					log.Printf("Giving up %d %s\n", res.StatusCode, req.URL)
				}
				return res, triperr
			}
			if res.StatusCode >= 200 && res.StatusCode < 400 {
				return res, triperr
			}
		}

		if tries > 10 {
			log.Printf("Too many tries\n")
			return res, triperr
		}
	}
}

var (
	metricAPICalls = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "evemarketwatch",
		Subsystem: "api",
		Name:      "calls",
		Help:      "API call statistics.",
		Buckets:   prometheus.ExponentialBuckets(10, 1.45, 20),
	},
		[]string{"host", "status", "try", "endpoint"},
	)

	metricAPIErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "evemarketwatch",
		Subsystem: "api",
		Name:      "errors",
		Help:      "Count of API errors.",
	})
)

func init() {
	prometheus.MustRegister(
		metricAPICalls,
		metricAPIErrors,
	)
}
