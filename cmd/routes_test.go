package main

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	ipqapi "github.com/akyriako/ipquery/api"
	"github.com/akyriako/ipquery/response"
)

func TestRoutes(t *testing.T) {
	asn, err := ipqapi.NewAsnReader("../geolite/GeoLite2-ASN.mmdb")
	if err != nil {
		t.Fatal(err)
	}
	defer asn.Close()
	city, err := ipqapi.NewCityReader("../geolite/GeoLite2-City.mmdb")
	if err != nil {
		t.Fatal(err)
	}
	defer city.Close()
	_, trusted, _ := net.ParseCIDR("172.30.0.0/16")
	cfg := response.DefaultConfig()
	s := &ipqapi.Server{LookupClient: &ipqapi.LookupClient{AsnReader: asn, CityReader: city, TrustedProxies: []*net.IPNet{trusted}}, ResponseConfig: &cfg}
	router := newRouter(s)
	strip := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	for _, tt := range []struct {
		path, ua, accept, remote, forwarded, wantIP, content, body string
		status                                                     int
	}{
		{path: "/health", body: "ok", content: "text/plain", status: 200},
		{path: "/own", remote: "8.8.8.8:1234", wantIP: "", body: "8.8.8.8", content: "text/plain", status: 200},
		{path: "/own?format=ansi", remote: "[2001:4860:4860::8888]:1234", body: "2001:4860:4860::8888", content: "text/plain", status: 200},
		{path: "/own/all", remote: "8.8.8.8:1234", wantIP: "8.8.8.8", content: "application/json", status: 200},
		{path: "/lookup/8.8.8.8", wantIP: "8.8.8.8", content: "application/json", status: 200},
		{path: "/lookup/2001:4860:4860::8888?format=pretty", wantIP: "2001:4860:4860::8888", content: "application/json", status: 200},
		{path: "/lookup/2001%3A4860%3A4860%3A%3A8888", wantIP: "2001:4860:4860::8888", content: "application/json", status: 200},
		{path: "/", ua: "curl/8.0", remote: "8.8.8.8:1234", wantIP: "8.8.8.8", content: "text/plain", status: 200},
		{path: "/?format=json", ua: "curl/8.0", remote: "8.8.8.8:1234", wantIP: "8.8.8.8", content: "application/json", status: 200},
		{path: "/", ua: "curl/8.0", accept: "application/json", remote: "8.8.8.8:1234", wantIP: "8.8.8.8", content: "application/json", status: 200},
		{path: "/", ua: "Mozilla/5.0", accept: "text/html", body: "<!doctype html>", content: "text/html", status: 200},
		{path: "/own/all", ua: "Mozilla/5.0", accept: "text/html", remote: "8.8.8.8:1234", wantIP: "8.8.8.8", content: "application/json", status: 200},
		{path: "/own/all", ua: "HTTPie/3.2", remote: "172.30.0.2:1234", forwarded: "2001:4860:4860::8888", wantIP: "2001:4860:4860::8888", content: "text/plain", status: 200},
		{path: "/own/all?format=json", remote: "8.8.8.8:1234", forwarded: "1.1.1.1", wantIP: "8.8.8.8", content: "application/json", status: 200},
		{path: "/lookup/invalid", body: "invalid ip", status: 400},
		{path: "/lookup/8.8.8.8?format=invalid", body: "format must be", status: 400},
		{path: "/?format=invalid", ua: "Mozilla/5.0", body: "format must be", status: 400},
		{path: "/missing", status: 404},
	} {
		t.Run(tt.path+tt.ua+tt.remote+tt.accept, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, tt.path, nil)
			r.RemoteAddr = tt.remote
			if r.RemoteAddr == "" {
				r.RemoteAddr = "8.8.8.8:1234"
			}
			r.Header.Set("User-Agent", tt.ua)
			r.Header.Set("Accept", tt.accept)
			r.Header.Set("X-Forwarded-For", tt.forwarded)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			if w.Code != tt.status || !strings.HasPrefix(w.Header().Get("Content-Type"), tt.content) || !strings.Contains(w.Body.String(), tt.body) {
				t.Fatalf("HTTP %d %s: %s", w.Code, w.Header(), w.Body)
			}
			if tt.wantIP == "" {
				return
			}
			data := strip.ReplaceAll(w.Body.Bytes(), nil)
			if tt.content == "application/json" && strings.Contains(w.Body.String(), "\x1b") {
				t.Fatal("ANSI leaked into JSON")
			}
			if tt.content == "text/plain" && !strings.Contains(w.Body.String(), "\x1b[36m") {
				t.Fatal("missing ANSI")
			}
			if !strings.HasSuffix(string(data), "\n") {
				t.Fatal("missing newline")
			}
			var result ipqapi.LookupResult
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatal(err)
			}
			if result.IP != tt.wantIP {
				t.Fatalf("IP %s", result.IP)
			}
			var schema map[string]json.RawMessage
			_ = json.Unmarshal(data, &schema)
			if len(schema) != 4 || schema["ip"] == nil || schema["isp"] == nil || schema["location"] == nil || schema["risk"] == nil {
				t.Fatalf("schema changed: %s", data)
			}
			for key, count := range map[string]int{"isp": 3, "location": 9, "risk": 6} {
				var fields map[string]any
				_ = json.Unmarshal(schema[key], &fields)
				if len(fields) != count {
					t.Fatalf("%s schema changed", key)
				}
			}
			vary := strings.Join(w.Header().Values("Vary"), ",")
			if !strings.Contains(vary, "Accept") || !strings.Contains(vary, "User-Agent") {
				t.Fatal("missing cache variation")
			}
		})
	}
	cfg.ColorEnabled = false
	r := httptest.NewRequest("GET", "/lookup/8.8.8.8?format=ansi", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if strings.Contains(w.Body.String(), "\x1b") || !json.Valid(w.Body.Bytes()) || !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
		t.Fatal("disabled colors failed")
	}
}
