package politeness

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestTransport_EnforcesMinDelayPerDomain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	transport := NewTransport(nil, "whatisgoingbot-test", 30*time.Millisecond, 10)
	client := &http.Client{Transport: transport}

	start := time.Now()
	for i := 0; i < 3; i++ {
		resp, err := client.Get(srv.URL + "/x")
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		resp.Body.Close()
	}
	elapsed := time.Since(start)

	// 3 requests with a 30ms minimum gap between each means at least
	// 2 gaps (60ms) must have elapsed.
	if elapsed < 55*time.Millisecond {
		t.Errorf("expected at least ~60ms for 3 requests at 30ms min delay, took %v", elapsed)
	}
}

func TestTransport_CapsConcurrencyPerDomain(t *testing.T) {
	var current, peak int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			// Deliberately exempt from the concurrency cap (see Transport),
			// and irrelevant to what this test measures.
			w.WriteHeader(http.StatusOK)
			return
		}

		n := atomic.AddInt32(&current, 1)
		for {
			p := atomic.LoadInt32(&peak)
			if n <= p || atomic.CompareAndSwapInt32(&peak, p, n) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		atomic.AddInt32(&current, -1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	const maxConcurrent = 2
	transport := NewTransport(nil, "whatisgoingbot-test", time.Microsecond, maxConcurrent)
	client := &http.Client{Transport: transport}

	done := make(chan struct{})
	for i := 0; i < 5; i++ {
		go func() {
			resp, err := client.Get(srv.URL + "/x")
			if err == nil {
				resp.Body.Close()
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 5; i++ {
		<-done
	}

	if atomic.LoadInt32(&peak) > maxConcurrent {
		t.Errorf("peak concurrent requests = %d, want <= %d", peak, maxConcurrent)
	}
}
