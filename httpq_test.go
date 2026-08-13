package main_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	httpq "github.com/FredrikHelander/httpq"
)

func TestPublish(t *testing.T) {
	q := httpq.NewHTTPQ()
	q.Timeout = 50 * time.Millisecond
	srv := httptest.NewServer(q.Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/topic1", "text/plain", strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusRequestTimeout {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusRequestTimeout)
	}
	if q.PubFails != 1 {
		t.Fatalf("PubFails = %d, want 1", q.PubFails)
	}
	if q.TxBytes != 0 {
		t.Fatalf("TxBytes = %d, want 0", q.TxBytes)
	}
}

func TestConsume(t *testing.T) {
	q := httpq.NewHTTPQ()
	q.Timeout = 50 * time.Millisecond
	srv := httptest.NewServer(q.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/topic1")
	if err != nil {
		t.Fatalf("consume: %v", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusRequestTimeout {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusRequestTimeout)
	}
	if q.SubFails != 1 {
		t.Fatalf("SubFails = %d, want 1", q.SubFails)
	}
	if q.RxBytes != 0 {
		t.Fatalf("RxBytes = %d, want 0", q.RxBytes)
	}
}

func TestPublishAndConsume(t *testing.T) {
	q := httpq.NewHTTPQ()
	q.Timeout = time.Second
	srv := httptest.NewServer(q.Handler())
	defer srv.Close()

	const msg = "hello 1"
	var (
		wg      sync.WaitGroup
		gotBody string
		gotCode int
		pubCode int
		pubErr  error
		consErr error
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		resp, err := http.Get(srv.URL + "/NhPvrxcJ5WfsYJ")
		if err != nil {
			consErr = err
			return
		}
		defer resp.Body.Close()
		gotCode = resp.StatusCode
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			consErr = err
			return
		}
		gotBody = string(b)
	}()
	go func() {
		defer wg.Done()
		// brief pause so the consumer is waiting first
		time.Sleep(20 * time.Millisecond)
		resp, err := http.Post(srv.URL+"/NhPvrxcJ5WfsYJ", "text/plain", strings.NewReader(msg))
		if err != nil {
			pubErr = err
			return
		}
		defer resp.Body.Close()
		io.Copy(io.Discard, resp.Body)
		pubCode = resp.StatusCode
	}()
	wg.Wait()

	if consErr != nil {
		t.Fatalf("consume: %v", consErr)
	}
	if pubErr != nil {
		t.Fatalf("publish: %v", pubErr)
	}
	if pubCode != http.StatusOK {
		t.Fatalf("publish status = %d, want %d", pubCode, http.StatusOK)
	}
	if gotCode != http.StatusOK {
		t.Fatalf("consume status = %d, want %d", gotCode, http.StatusOK)
	}
	if gotBody != msg {
		t.Fatalf("consume body = %q, want %q", gotBody, msg)
	}
	if q.TxBytes != len(msg) {
		t.Fatalf("TxBytes = %d, want %d", q.TxBytes, len(msg))
	}
	if q.RxBytes != len(msg) {
		t.Fatalf("RxBytes = %d, want %d", q.RxBytes, len(msg))
	}
	if q.PubFails != 0 || q.SubFails != 0 {
		t.Fatalf("fails = pub:%d sub:%d, want 0", q.PubFails, q.SubFails)
	}
}
