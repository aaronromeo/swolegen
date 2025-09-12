package llm

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// fetchToTmp downloads text and writes it to a temp file. Returns path and content.
func fetchToTmp(ctx context.Context, url, prefix string, maxFetchBytes int) (string, string, error) {
	content, err := fetchText(ctx, url, maxFetchBytes)
	if err != nil {
		return "", "", err
	}
	f, err := os.CreateTemp("", "swolegen-"+prefix+"-*.txt")
	if err != nil {
		return "", content, err
	}
	defer func() {
		f.Close()           //nolint:errcheck
		os.Remove(f.Name()) //nolint:errcheck
	}()
	if _, err := f.WriteString(content); err != nil {
		return f.Name(), content, err
	}
	return f.Name(), content, nil
}

// fetchText downloads the content at a URL and returns it as a string.
// It supports http(s) and file URLs; for empty or invalid URLs, returns empty string.
func fetchText(ctx context.Context, url string, maxFetchBytes int) (string, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return "", nil
	}
	// Local files support (file:// or relative path)
	if strings.HasPrefix(url, "file://") {
		p := strings.TrimPrefix(url, "file://")
		if strings.HasPrefix(url, "file://") {
			if pp, ok := strings.CutPrefix(url, "file://"); ok {
				p = pp
			}
		}
		f, err := os.Open(p)
		if err != nil {
			return "", err
		}
		defer f.Close() //nolint:errcheck
		lr := &io.LimitedReader{R: f, N: int64(maxFetchBytes)}
		b, err := io.ReadAll(lr)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		// treat as local path
		f, err := os.Open(url)
		if err != nil {
			return "", err
		}
		defer f.Close() //nolint:errcheck
		lr := &io.LimitedReader{R: f, N: int64(maxFetchBytes)}
		b, err := io.ReadAll(lr)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("GET %s: %d", url, resp.StatusCode)
	}
	lr := &io.LimitedReader{R: resp.Body, N: int64(maxFetchBytes)}
	b, err := io.ReadAll(lr)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// indentForBlock indents each line by two spaces for YAML literal blocks.
func indentForBlock(s string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = "  " + lines[i]
	}

	return strings.Join(lines, "\n")
}
