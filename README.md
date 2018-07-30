# eve-marketwatch

Self contained microservice to collect market data and stream all additions, changes, and deletions over a websocket. On connect a dump of the current state will be sent (it is quite big) followed by a stream of all changes.

This can be used to keep a database synchronized with the current market state, try to estimate completed orders for history, track players persistently making frequent changes (*cough* bots *cough*), or to find high value items sold to try to gank the player later. The possibilities are endless!

The microservice will spawn one goroutine (think lightweight thread) per market and collect the next available set of data when the cache time expires. Pages are also concurrently pulled with a concurrency limit of 100 requests in flight to keep everything to one https connection. 

## dockerized

`docker run antihax/eve-marketwatch -p 3005:3005`

## compilation

* Build the cmd directory.

## environment

You can optionally pass an SSO configuration and a refresh_token from CCP to also gather market information from public structures. This requires the esi-markets.structure_markets.v1 scope. You can register an application to receive the clientID and secret at CCP's [Third Party Applications](https://developers.eveonline.com/) site.

You will need to use another tool to obtain a refresh_token using the clientID and secret to provide this tool.

| Variable        | Description | 
| ------------- |-------------| 
| ESI_CLIENTID_TOKENSTORE | SSO ClientID |
| ESI_SECRET_TOKENSTORE | SSO Secret |
| ESI_REFRESHKEY | a refresh_token from the ClientID and Secret above |

Note: turning on structures will cause an initial performance hit as the service discovers which structures actually have a market. The consumer will spew errors and hit the error limit, but after an hour, this should settle and then operate smoothly.

## operation

Connect to the websocket on port 3005 `ws://address:3005/` and receive a stream of JSON data of market changes. On initial connect, you will receive a dump of the current market state.

Recommendation is to read messages asap and put them into queues so as not to hit timeout states on the websocket.

The `:3000` port has prometheus stats and golang pprof information. This port should not be exposed, please protect it.

## data received

Data will be encapsulated in a json frame. 
```json
{"action": "actionstring", "payload": { json payload }}
``` 
Payloads are as follows

### addition

ESI formatted [market orders](https://esi.evetech.net/ui/#/Market/get_markets_region_id_orders)

### change and deletion

Changes and deletions will be an array of the following types. Deletions will also have a volume_remain of 0.

```golang
	int64     `json:"order_id"`
	int64     `json:"location_id"`
	int32     `json:"type_id"`
	int32     `json:"volume_change,omitempty"`
	int32     `json:"volume_remain,omitempty"`
	float64   `json:"price"`
	int32     `json:"duration,omitempty"`
    bool      `json:"is_buy_order,omitempty"`
    time.Time `json:"issued,omitempty"`
	time.Time `json:"time_changed"`
``` 

