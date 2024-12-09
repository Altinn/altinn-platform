package utils

import (
	"io"
	"net/url"
)

func isUrl(s string) bool {
	_, err := url.ParseRequestURI(s)
	return err == nil
}

func closeIgnoreError(c io.Closer) {
	_ = c.Close()
}
