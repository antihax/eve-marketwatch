package wsbroadcast

import (
	"net"
	"net/http"
	"net/url"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

func TestWebSocket(t *testing.T) {
	// Setup a new hub
	hub := NewHub()

	hub.OnRegister(func() interface{} {
		return "sup"
	})

	go hub.Run()

	// Run a webserver for the socket
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		hub.ServeWs(w, r)
	})

	// Listen on random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	assert.Nil(t, err)

	// Get the address
	addr := listener.Addr()
	go func() { panic(http.Serve(listener, nil)) }()

	// Setup a client for testing
	u := url.URL{Scheme: "ws", Host: addr.String(), Path: "/"}

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	assert.Nil(t, err)
	message := ""
	err = c.ReadJSON(&message)
	assert.Nil(t, err)

	assert.Equal(t, "sup", message)

	err = c.Close()
	assert.Nil(t, err)
}
