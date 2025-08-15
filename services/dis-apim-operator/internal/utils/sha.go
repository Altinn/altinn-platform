package utils

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
)

// Sha256FromUrlContent returns the SHA256 hash of the content at the given URL.
func sha256FromURLContent(ctx context.Context, url string) (string, error) {
	resp, err := getContentURL(ctx, url)
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

// Sha256FromContent returns the SHA256 hash of the given content. If the content is a URL, it will fetch the content and return the SHA256 hash.
func Sha256FromContent(ctx context.Context, content *string) (string, error) {
	if content == nil {
		return "", nil
	}
	if isURL(*content) {

		return sha256FromURLContent(ctx, *content)
	}
	h := sha256.New()
	h.Write([]byte(*content))
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
