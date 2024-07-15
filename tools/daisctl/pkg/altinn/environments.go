package altinn

import "github.com/altinn/altinn-platform/daisctl/pkg/util"

type Environment struct {
	PlatformUrl    string `json:"platformUrl"`
	Hostname       string `json:"hostname"`
	AppPrefix      string `json:"appPrefix"`
	PlatformPrefix string `json:"platformPrefix"`
	Name           string `json:"name"`
	Type           string `json:"type"`
}

type EnvsResp struct {
	Environments []Environment `json:"environments"`
}

func GetEnvironments(url string) ([]Environment, error) {
	response, err := util.RequestObject[EnvsResp](url)
	if err != nil {
		return nil, err
	}
	return response.Environments, nil
}
