package marketwatch

import (
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/antihax/eve-marketwatch/wsbroadcast"

	"github.com/antihax/goesi"
	"golang.org/x/oauth2"
)

// MarketWatch provides CCP Market Data
type MarketWatch struct {

	// goesi client
	esi *goesi.APIClient

	// websocket handler
	broadcast *wsbroadcast.Hub

	// authentication
	doAuth    bool
	token     *oauth2.TokenSource
	tokenAuth *goesi.SSOAuthenticator

	// data store
	market     map[int64]*sync.Map
	structures map[int64]*Structure
	mmutex     sync.RWMutex
	smutex     sync.RWMutex
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
	if tokenClientID == "" || tokenSecret == "" || refresh == "" {
		log.Println("Warning: Missing authentication parameters so only regional market will be polled")
	}
	auth := goesi.NewSSOAuthenticator(httpclient, tokenClientID, tokenSecret, "", []string{})

	tok := &oauth2.Token{
		Expiry:       time.Now(),
		AccessToken:  "",
		RefreshToken: refresh,
		TokenType:    "Bearer",
	}

	// Build our private token
	doAuth := false
	token, err := auth.TokenSource(tok)
	if err != nil {
		log.Println("Warning: Failed to authenticate refresh_token so only regional market will be polled")
	} else {
		doAuth = true
	}

	return &MarketWatch{
		// ESI Client
		esi: goesi.NewAPIClient(
			httpclient,
			"eve-marketwatch",
		),

		// Websocket Broadcaster
		broadcast: wsbroadcast.NewHub(),

		// ESI SSO Handler
		doAuth:    doAuth,
		token:     &token,
		tokenAuth: auth,

		// Market Data Map
		market:     make(map[int64]*sync.Map),
		structures: make(map[int64]*Structure),
	}
}

// Run starts listening on port 3005 for API requests
func (s *MarketWatch) Run() error {

	// Setup the callback to send the market to the client on connect
	s.broadcast.OnRegister(s.dumpMarket)
	go s.broadcast.Run()

	go s.startUpMarketWorkers()

	// Handler for the websocket
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		s.broadcast.ServeWs(w, r)
	})

	return http.ListenAndServe(":3005", nil)
}
