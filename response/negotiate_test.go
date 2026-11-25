package response

import (
	"net/http/httptest"
	"testing"
)

func TestResolve(t *testing.T) {
	for _, tt := range []struct{ ua, accept, query, want string }{
		{"curl/8.0", "*/*", "", "ansi"}, {"HTTPie/3.2", "", "", "ansi"}, {"Wget/1.21", "*/*", "", "ansi"}, {"xh/0.23", "", "", "ansi"},
		{"Mozilla/5.0", "text/html,application/xhtml+xml,*/*;q=0.8", "", "pretty"},
		{"Go-http-client/1.1", "*/*", "", "json"}, {"python-requests/2.32", "", "", "json"}, {"", "", "", "json"},
		{"curl/8.0", "application/json", "", "json"}, {"HTTPie/3.2", "application/json, */*;q=0.5", "", "json"},
		{"curl/8.0", "application/json;q=0.2,text/plain;q=0.8", "", "ansi"},
		{"curl/8.0", "text/plain;q=0,*/*", "", "json"}, {"curl/8.0", "application/xml", "", "json"},
		{"curl/8.0", "*/*", "?format=json", "json"}, {"curl/8.0", "*/*", "?format=pretty", "pretty"},
		{"Mozilla/5.0", "application/json", "?format=ansi", "ansi"}, {"curl/8.0", "*/*", "?format=auto", "ansi"},
		{"fake curl/8.0", "*/*", "", "json"},
	} {
		t.Run(tt.ua+tt.accept+tt.query, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/"+tt.query, nil)
			r.Header.Set("User-Agent", tt.ua)
			r.Header.Set("Accept", tt.accept)
			got, err := Resolve(r, DefaultConfig())
			if err != nil || got != tt.want {
				t.Fatalf("got %q %v want %q", got, err, tt.want)
			}
		})
	}
	for _, q := range []string{"?format=", "?format=invalid", "?format=json&format=ansi"} {
		if _, err := Resolve(httptest.NewRequest("GET", "/"+q, nil), DefaultConfig()); err == nil {
			t.Fatal("invalid query accepted")
		}
	}
	c := DefaultConfig()
	c.DefaultFormat = "pretty"
	r := httptest.NewRequest("GET", "/?format=auto", nil)
	r.Header.Set("User-Agent", "curl/8")
	if mode, _ := Resolve(r, c); mode != "ansi" {
		t.Fatal("explicit auto did not override default")
	}
	c.ColorEnabled = false
	if mode, _ := Resolve(r, c); mode != "pretty" {
		t.Fatal("color disabled ignored")
	}
	r = httptest.NewRequest("GET", "/?format=ansi", nil)
	if mode, _ := Resolve(r, c); mode != "pretty" {
		t.Fatal("color switch ignored for explicit ANSI")
	}
}
