package factory

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gepis/sm-gh-api/api"
	"github.com/gepis/sm-gh-api/core/ghinstance"
	"github.com/gepis/sm-gh-api/core/httpunix"
	"github.com/gepis/sm-gh-api/pkg/iostreams"
)

var timezoneNames = map[int]string{
	-39600: "Pacific/Niue",
	-36000: "Pacific/Honolulu",
	-34200: "Pacific/Marquesas",
	-32400: "America/Anchorage",
	-28800: "America/Los_Angeles",
	-25200: "America/Chihuahua",
	-21600: "America/Chicago",
	-18000: "America/Bogota",
	-14400: "America/Caracas",
	-12600: "America/St_Johns",
	-10800: "America/Argentina/Buenos_Aires",
	-7200:  "Atlantic/South_Georgia",
	-3600:  "Atlantic/Cape_Verde",
	0:      "Europe/London",
	3600:   "Europe/Amsterdam",
	7200:   "Europe/Athens",
	10800:  "Europe/Istanbul",
	12600:  "Asia/Tehran",
	14400:  "Asia/Dubai",
	16200:  "Asia/Kabul",
	18000:  "Asia/Tashkent",
	19800:  "Asia/Kolkata",
	20700:  "Asia/Kathmandu",
	21600:  "Asia/Dhaka",
	23400:  "Asia/Rangoon",
	25200:  "Asia/Bangkok",
	28800:  "Asia/Manila",
	31500:  "Australia/Eucla",
	32400:  "Asia/Tokyo",
	34200:  "Australia/Darwin",
	36000:  "Australia/Brisbane",
	37800:  "Australia/Adelaide",
	39600:  "Pacific/Guadalcanal",
	43200:  "Pacific/Nauru",
	46800:  "Pacific/Auckland",
	49500:  "Pacific/Chatham",
	50400:  "Pacific/Kiritimati",
}

type configGetter interface {
	Get(string, string) (string, error)
}

// generic authenticated HTTP client for commands
func NewHTTPClient(io *iostreams.IOStreams, cfg configGetter, appVersion string, setAccept bool) (*http.Client, error) {
	var opts []api.ClientOption

	unixSocket, err := cfg.Get("", "http_unix_socket")
	if err != nil {
		return nil, err
	}

	if unixSocket != "" {
		opts = append(opts, api.ClientOption(func(http.RoundTripper) http.RoundTripper {
			return httpunix.NewRoundTripper(unixSocket)
		}))
	}

	if verbose := os.Getenv("DEBUG"); verbose != "" {
		logTraffic := strings.Contains(verbose, "api")
		opts = append(opts, api.VerboseLog(io.ErrOut, logTraffic, io.IsStderrTTY()))
	}

	opts = append(opts,
		api.AddHeader("User-Agent", fmt.Sprintf("GitHub API %s", appVersion)),
		api.AddHeaderFunc("Authorization", func(req *http.Request) (string, error) {
			hostname := ghinstance.NormalizeHostname(getHost(req))
			if token, err := cfg.Get(hostname, "oauth_token"); err == nil && token != "" {
				return fmt.Sprintf("token %s", token), nil
			}
			return "", nil
		}),
		api.AddHeaderFunc("Time-Zone", func(req *http.Request) (string, error) {
			if req.Method != "GET" && req.Method != "HEAD" {
				if time.Local.String() != "Local" {
					return time.Local.String(), nil
				}
				_, offset := time.Now().Zone()
				return timezoneNames[offset], nil
			}
			return "", nil
		}),
	)

	if setAccept {
		opts = append(opts,
			api.AddHeaderFunc("Accept", func(req *http.Request) (string, error) {
				accept := "application/vnd.github.merge-info-preview+json" // PullRequest.mergeStateStatus
				accept += ", application/vnd.github.nebula-preview" // visibility when RESTing repos into
				if ghinstance.IsEnterprise(getHost(req)) {
					accept += ", application/vnd.github.antiope-preview"    // Commit.statusCheckRollup
					accept += ", application/vnd.github.shadow-cat-preview" // PullRequest.isDraft
				}
				return accept, nil
			}),
		)
	}

	return api.NewHTTPClient(opts...), nil
}

func getHost(r *http.Request) string {
	if r.Host != "" {
		return r.Host
	}

	return r.URL.Hostname()
}
