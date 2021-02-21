package throttle

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	timeout = 5 * time.Second
)

// Throttle allows the client to throttle the rate at which requests are
// handled by clients.
type Throttle struct {
	requestRate     time.Duration
	tokenChansMutex sync.Mutex
	tokenChans      map[string]chan struct{}
}

// New creates a new Throttle with the given request rate.
func New(requestRate time.Duration) *Throttle {
	throttle := Throttle{
		requestRate: requestRate,
		tokenChans:  make(map[string]chan struct{}),
	}
	return &throttle
}

// Wait ensures that only one request per client is served within Throttle's
// defined timeout. For every client, which is distinguished by its IPv4
// address, a token is produced once per timeout. The token is given to one of
// the waiting requests, and a new token is produced thereafter. A request
// either acquires a token within the given timeout, and the request is
// returned; or the request runs out of time, and an error is returned. The
// first token is spawned immediately.
func (t *Throttle) Wait(r *http.Request) (*http.Request, error) {
	// every user has a channel that gets tokens
	user := ip4(r)
	t.tokenChansMutex.Lock()
	tokenChan, ok := t.tokenChans[user]
	if !ok {
		tokenChan = make(chan struct{})
		go func() {
			// the first token is spawned immediately
			tokenChan <- struct{}{}
		}()
		t.tokenChans[user] = tokenChan
	}
	t.tokenChansMutex.Unlock()

	// timeout after given time
	timeoutChan := make(chan struct{})
	go func() {
		time.Sleep(t.requestRate)
		timeoutChan <- struct{}{}
	}()

	// wait for timeout or token
	select {
	case <-tokenChan:
		// token acquired: request can be served, new token be spawned
		go func() {
			time.Sleep(t.requestRate)
			tokenChan <- struct{}{}
		}()
		return r, nil
	case <-timeoutChan:
		// timeout: do not serve the request
		return nil, fmt.Errorf("one request per %v allowed", timeout)
	}
}

func ip4(r *http.Request) string {
	if !strings.Contains(r.RemoteAddr, ":") {
		return r.RemoteAddr
	}
	fields := strings.Split(r.RemoteAddr, ":")
	if len(fields) < 2 {
		return r.RemoteAddr
	}
	return fields[0]
}
