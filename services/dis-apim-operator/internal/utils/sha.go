package utils

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

func Sha256FromUrlContent(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer closeIgnoreError(resp.Body)

	h := sha256.New()
	if _, err := io.Copy(h, resp.Body); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func Sha256FromContent(content string) (string, error) {
	if isUrl(content) {
		return Sha256FromUrlContent(content)
	}
	h := sha256.New()
	h.Write([]byte(content))
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func isUrl(s string) bool {
	_, err := url.ParseRequestURI(s)
	return err == nil
}

func closeIgnoreError(c io.Closer) {
	_ = c.Close()
}
