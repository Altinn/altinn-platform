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
	_, err := url.ParseRequestURI(s)
	return err == nil
}

func getContentUrl(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}
	if resp.ContentLength > maxContentSize {
		return nil, fmt.Errorf("content size exceeds the maximum allowed size of %d bytes, actual size %d", maxContentSize, resp.ContentLength)
	}
	return resp, nil
}

func closeIgnoreError(c io.Closer) {
	_ = c.Close()
}
