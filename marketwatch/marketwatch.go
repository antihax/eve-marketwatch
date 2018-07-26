package marketwatch

import (
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/antihax/goesi"
	"golang.org/x/oauth2"
)

// MarketWatch provides CCP Market Data
type MarketWatch struct {

	// goesi client
	esi *goesi.APIClient

	// authentication
	token     *oauth2.TokenSource
	tokenAuth *goesi.SSOAuthenticator
	market    map[int64]*sync.Map
}

// NewMarketWatch creates a new MarketWatch microservice
func NewMarketWatch(refresh, tokenClientID, tokenSecret string) *MarketWatch {
	httpclient := &http.Client{
		Transport: &ApiTransport{
			next: &http.Transport{
				MaxIdleConns: 200,
				DialContext: (&net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 5 * 60 * time.Second,
					DualStack: true,
				}).DialContext,
				IdleConnTimeout:       5 * 60 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 15 * time.Second,
				ExpectContinueTimeout: 0,
				MaxIdleConnsPerHost:   20,
			},
		},
	}

	// Setup an authenticator for our user tokens
	auth := goesi.NewSSOAuthenticator(httpclient, tokenClientID, tokenSecret, "", []string{})

	tok := &oauth2.Token{
		Expiry:       time.Now(),
		AccessToken:  "",
		RefreshToken: refresh,
		TokenType:    "Bearer",
	}

	// Build our private token
	token, err := auth.TokenSource(tok)
	if err != nil {
		log.Fatalln(err)
	}

	return &MarketWatch{
		// ESI Client
		esi: goesi.NewAPIClient(
			httpclient,
			"eve-marketwatch",
		),

		// ESI SSO Handler
		token:     &token,
		tokenAuth: auth,

		// Market Data Map
		market: make(map[int64]*sync.Map),
	}
}

// Run starts listening on port 3005 for API requests
func (s *MarketWatch) Run() error {

	go s.startUpMarketWorkers()

	//http.HandleFunc("/killmail", s.killmailHandler)
	return http.ListenAndServe(":3005", nil)
}
