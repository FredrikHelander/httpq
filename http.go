package main

import (
	"net/http"
	"time"
	"sync"
	"io"
	"encoding/json"

	"github.com/go-chi/chi"
)

const timeout = 10 * time.Second

type HTTPQ struct {
	mu 		 sync.Mutex
	topics   map[string]chan []byte
	Timeout  time.Duration
	RxBytes  int // number of bytes (message body) consumed
	TxBytes  int // number of bytes (message body) published
	PubFails int // number of publish failures
	SubFails int // number of subscribe failures
}

func NewHTTPQ() *HTTPQ {
	return &HTTPQ{
		topics:  make(map[string]chan []byte),
		Timeout: timeout,
	}
}

// timeout returns how long Publish/Consume wait for the other.
func (h *HTTPQ) timeout() time.Duration {
	if h.Timeout > 0 {
		return h.Timeout
	}
	return timeout
}

// topic returns the channel for a topic name, creating it if needed.
// The mutex only protects the map — never hold it while sending/receiving on ch.
func (h *HTTPQ) topic(name string) chan []byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	ch, ok := h.topics[name]
	if !ok {
		ch = make(chan []byte) // unbuffered: send and receive must meet
		h.topics[name] = ch
	}
	return ch
}

type statsResponse struct {
	PubFails int `json:"pub_fails"`
	SubFails int `json:"sub_fails"`
	RxBytes  int `json:"rx_bytes"`
	TxBytes  int `json:"tx_bytes"`
}

func (h *HTTPQ) Handler() http.Handler {
	r := chi.NewRouter()

	r.Get("/stats", h.Stats().ServeHTTP)
	r.Get("/{topic}", h.Consume().ServeHTTP)
	r.Post("/{topic}", h.Publish().ServeHTTP)

	return r
}

func (h *HTTPQ) Stats() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		stats := statsResponse{
			PubFails: h.PubFails,
			SubFails: h.SubFails,
			RxBytes:  h.RxBytes,
			TxBytes:  h.TxBytes,
		}
		h.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	})
}

func (h *HTTPQ) Publish() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		topic := chi.URLParam(r, "topic")
		ch := h.topic(topic)

		timer := time.NewTimer(h.timeout())
		defer timer.Stop()

		select {
		case ch <- body:
			h.mu.Lock()
			h.TxBytes += len(body)
			h.mu.Unlock()
			w.WriteHeader(http.StatusOK)

		case <-timer.C:
			h.mu.Lock()
			h.PubFails++
			h.mu.Unlock()
			http.Error(w, "timeout waiting for consumer", http.StatusRequestTimeout)

		case <-r.Context().Done():
			h.mu.Lock()
			h.PubFails++
			h.mu.Unlock()
		}
	})
}

func (h *HTTPQ) Consume() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		topic := chi.URLParam(r, "topic")
		ch := h.topic(topic)

		timer := time.NewTimer(h.timeout())
		defer timer.Stop()

		select {
		case msg := <-ch:
			h.mu.Lock()
			h.RxBytes += len(msg)
			h.mu.Unlock()
			w.Write(msg)

		case <-timer.C:
			h.mu.Lock()
			h.SubFails++
			h.mu.Unlock()
			http.Error(w, "timeout waiting for producer", http.StatusRequestTimeout)

		case <-r.Context().Done():
			h.mu.Lock()
			h.SubFails++
			h.mu.Unlock()
		}
	})
}
