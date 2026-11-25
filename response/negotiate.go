package response

import (
	"fmt"
	"mime"
	"net/http"
	"strconv"
	"strings"
)

// quality honors specific exclusions before wildcard media ranges.
func quality(accept, target string) float64 {
	if strings.TrimSpace(accept) == "" {
		return 1
	}
	best, q := -1, 0.0
	for _, part := range strings.Split(accept, ",") {
		media, params, err := mime.ParseMediaType(strings.TrimSpace(part))
		if err != nil {
			continue
		}
		specific := -1
		switch {
		case media == target:
			specific = 2
		case media == strings.SplitN(target, "/", 2)[0]+"/*":
			specific = 1
		case media == "*/*":
			specific = 0
		}
		if specific < 0 {
			continue
		}
		v := 1.0
		if raw, ok := params["q"]; ok {
			v, err = strconv.ParseFloat(raw, 64)
			if err != nil || v < 0 || v > 1 {
				continue
			}
		}
		if specific > best || specific == best && v > q {
			best, q = specific, v
		}
	}
	return q
}

func IsCLI(r *http.Request) bool {
	ua := strings.ToLower(r.UserAgent())
	for _, prefix := range []string{"curl/", "httpie/", "wget/", "xh/"} {
		if strings.HasPrefix(ua, prefix) {
			return true
		}
	}
	return false
}

func IsBrowser(r *http.Request) bool {
	return strings.HasPrefix(strings.ToLower(r.UserAgent()), "mozilla/")
}

func wantsJSON(accept string) bool {
	for _, part := range strings.Split(accept, ",") {
		media, _, err := mime.ParseMediaType(strings.TrimSpace(part))
		if err == nil && media == "application/json" && quality(accept, "application/json") > 0 {
			return true
		}
	}
	return false
}

// Landing keeps the existing browser UI on /; explicit formats always select data.
func Landing(r *http.Request) bool {
	return !r.URL.Query().Has("format") && IsBrowser(r) && !wantsJSON(r.Header.Get("Accept")) && quality(r.Header.Get("Accept"), "text/html") > 0
}

func Resolve(r *http.Request, c Config) (string, error) {
	mode := c.DefaultFormat
	if values, ok := r.URL.Query()["format"]; ok {
		if len(values) != 1 || !validFormat(values[0]) {
			return "", fmt.Errorf("format must be json, pretty, ansi, or auto")
		}
		mode = values[0]
	}
	if mode == "auto" {
		accept := r.Header.Get("Accept")
		switch {
		case wantsJSON(accept) && quality(accept, "application/json") >= quality(accept, "text/plain") && quality(accept, "application/json") >= quality(accept, "text/html"):
			mode = "json"
		case IsCLI(r) && quality(accept, "text/plain") > 0:
			mode = "ansi"
		case IsBrowser(r):
			mode = "pretty"
		default:
			mode = "json"
		}
	}
	if mode == "ansi" && !c.ColorEnabled {
		mode = "pretty"
	}
	return mode, nil
}
