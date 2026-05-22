package kube

import (
	"github.com/altinn/altinn-platform/disctl/pkg/util"
)

type AppInfo struct {
	Version string `json:"version"`
	Release string `json:"release"`
}

type AppVersions struct {
	AppName  string
	Versions map[string]string // Map of environment to version
}

func GetAppInfos(url string) ([]AppInfo, error) {
	r, err := util.RequestArray[AppInfo](url)
	if err != nil {
		return nil, err
	}
	return r, nil
}
