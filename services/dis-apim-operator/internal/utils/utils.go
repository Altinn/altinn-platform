package utils

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
	},
}

func isUrl(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && len(u.Host) > 0
}

func getContentUrl(ctx context.Context, urlInput string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlInput, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer closeIgnoreError(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}
	if resp.ContentLength > maxContentSize {
		defer closeIgnoreError(resp.Body)
		return nil, fmt.Errorf("content size exceeds the maximum allowed size of %d bytes, actual size %d", maxContentSize, resp.ContentLength)
	}
	return resp, nil
}

func closeIgnoreError(c io.Closer) {
	_ = c.Close()
}
