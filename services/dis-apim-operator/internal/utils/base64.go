package utils

import (
	"encoding/base64"
	"io"
)

// Base64FromUrlContent returns the base64 encoding of the content at the given URL.
func base64FromUrlContent(url string) (string, error) {
	resp, err := getContentUrl(url)
	if err != nil {
		return "", err
	}
	defer closeIgnoreError(resp.Body)

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// Base64FromContent returns the base64 encoding of the given content. If the content is a URL, it will fetch the content and return the base64 encoding.
func Base64FromContent(content *string) (string, error) {
	if content == nil {
		return "", nil
	}
	if isUrl(*content) {
		return base64FromUrlContent(*content)
	}
	return base64.StdEncoding.EncodeToString([]byte(*content)), nil
}
