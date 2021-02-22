package throttle

import (
	"fmt"
	"sync"
	"time"
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

// Wait ensures that only one request per client is allowed within Throttle's
// defined timeout. For every client, a token is produced once per timeout. The
// token is given to one of the waiting requests, and a new token is produced
// thereafter. A request either acquires a token within the given timeout, or
// the request runs out of time, and an error is returned. The first token is
// spawned immediately.
func (t *Throttle) Wait(client string) error {
	// every user has a channel that gets tokens
	t.tokenChansMutex.Lock()
	tokenChan, ok := t.tokenChans[client]
	if !ok {
		tokenChan = make(chan struct{})
		go func() {
			// the first token is spawned immediately
			tokenChan <- struct{}{}
		}()
		t.tokenChans[client] = tokenChan
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
		return nil
	case <-timeoutChan:
		// timeout: do not serve the request
		return fmt.Errorf("one request per %v allowed", t.requestRate)
	}
}
