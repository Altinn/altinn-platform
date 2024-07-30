package util

import (
	"encoding/json"
	"net/http"
)

// Generic request function to handle a single object
func RequestObject[T any](url string) (T, error) {
	res, err := http.Get(url)
	if err != nil {
		var zeroValue T
		return zeroValue, err
	}
	defer res.Body.Close()

	var respInfo T
	err = json.NewDecoder(res.Body).Decode(&respInfo)
	if err != nil {
		var zeroValue T
		return zeroValue, err
	}

	return respInfo, nil
}

// Generic request function to handle slices directly
func RequestArray[T any](url string) ([]T, error) {
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var respInfo []T
	err = json.NewDecoder(res.Body).Decode(&respInfo)
	if err != nil {
		return nil, err
	}

	return respInfo, nil
}
