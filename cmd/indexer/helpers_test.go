package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
)

func mustBuildRequest(rawurl string) *http.Request {
	parsed, _ := url.Parse(rawurl)
	r := httptest.NewRequest("GET", rawurl, nil)
	r.URL = parsed
	r.RequestURI = rawurl
	return r
}
