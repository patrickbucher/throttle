package throttle

import (
	"net/http"
	"sync"
	"testing"
	"time"
)

type TestCase struct {
	AllowedRate             time.Duration
	ProgressiveRequestPause time.Duration
	TotalRequests           int
	ExpectedRequestsOK      int
	ExpectedRequestsFail    int
}

var testCases = []TestCase{
	{
		// 1st request ready immediately
		// 2nd request allowed thereafter
		// 3rd request timed out
		// all further requests timed out (no more tokens spawned)
		AllowedRate:             1 * time.Second,
		ProgressiveRequestPause: 100 * time.Millisecond,
		TotalRequests:           10,
		ExpectedRequestsOK:      2,
		ExpectedRequestsFail:    8,
	},
	{
		// fresh token spawned between requests
		AllowedRate:             10 * time.Millisecond,
		ProgressiveRequestPause: 100 * time.Millisecond,
		TotalRequests:           10,
		ExpectedRequestsOK:      10,
		ExpectedRequestsFail:    0,
	},
	{
		// difference between token needed and token spawned must be 1000ms max
		// needed:  0,  450, 550+900=1450, 550+1350=1900
		// spawned: 0, 1000,         2000,          3000
		// diff:    0   550,          550,          1100 [timeout]
		AllowedRate:             1 * time.Second,
		ProgressiveRequestPause: 450 * time.Millisecond,
		TotalRequests:           5,
		ExpectedRequestsOK:      3,
		ExpectedRequestsFail:    2,
	},
}

func TestDoubleRequestTooFast(t *testing.T) {
	var okMu sync.Mutex
	var ok int

	var failMu sync.Mutex
	var fail int

	for _, testCase := range testCases {
		throttle := New(testCase.AllowedRate)

		var wg sync.WaitGroup

		ok, fail = 0, 0

		for i := 0; i < testCase.TotalRequests; i++ {
			wg.Add(1)
			go func(nthRequest int) {
				time.Sleep(time.Duration(nthRequest) * testCase.ProgressiveRequestPause)
				r, err := http.NewRequest(http.MethodGet, "http://localhost", nil)
				r, err = throttle.Wait(r)
				if r != nil && err == nil {
					okMu.Lock()
					ok++
					okMu.Unlock()
				} else {
					failMu.Lock()
					fail++
					failMu.Unlock()
				}
				wg.Done()
			}(i)
		}

		wg.Wait()
		if ok != testCase.ExpectedRequestsOK && fail != testCase.ExpectedRequestsFail {
			t.Errorf("ok/fail: expected %d/%d, got %d/%d",
				testCase.ExpectedRequestsOK, testCase.ExpectedRequestsFail, ok, fail)
		}
	}
}
