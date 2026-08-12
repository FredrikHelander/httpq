package main

import "net/http"

func main() {
	httpQ := NewHTTPQ()
	http.ListenAndServe(":23411", httpQ.Handler())
}
