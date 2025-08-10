package history

import (
	"context"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/hashicorp/go-retryablehttp"
)

var lineRx = regexp.MustCompile(`(?i)^(\d{4}-\d{2}-\d{2}).*?([A-Za-z][A-Za-z0-9 \-/()]+).*?(\d+(?:\.\d+)?).*?(\d{1,2})\s*reps?`)

type Entry struct {
	Date     string
	Exercise string
	LoadRaw  string
	RepsRaw  string
}

type DomainHistory struct {
	Entries []Entry
}

func FetchURL(ctx context.Context, url string) ([]byte, error) {
	c := retryablehttp.NewClient()
	req, _ := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, errors.New("bad status")
	}
	return io.ReadAll(resp.Body)
}

func ParseHistoryMarkdown(raw []byte) (DomainHistory, error) {
	var h DomainHistory
	for _, ln := range strings.Split(string(raw), "\n") {
		m := lineRx.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		h.Entries = append(h.Entries, Entry{Date: m[1], Exercise: m[2], LoadRaw: m[3], RepsRaw: m[4]})
	}
	return h, nil
}
