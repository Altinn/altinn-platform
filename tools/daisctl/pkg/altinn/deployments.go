package altinn

import (
	"fmt"

	"github.com/altinn/altinn-platform/daisctl/pkg/kube"
	"github.com/altinn/altinn-platform/daisctl/pkg/util"
)

type Deployments struct {
	Apps map[string]*kube.AppVersions
}

func newDeployments() *Deployments {
	return &Deployments{
		Apps: make(map[string]*kube.AppVersions),
	}

}

func GetAllDeployments() (*Deployments, error) {
	d := newDeployments()

	envs, err := GetEnvironments(util.EnvironmentsAPI)
	if err != nil {
		return nil, err
	}

	for _, e := range envs {
		kwUrl := fmt.Sprintf("%s/"+util.KubeWrapperAPI+"/Deployments", e.PlatformUrl)
		err := d.initAppsData(kwUrl, e)
		if err != nil {
			return nil, err
		}

		//TODO: in the near future there should not be a Daemonsets endpoint
		kwUrl = fmt.Sprintf("%s/"+util.KubeWrapperAPI+"/Daemonsets", e.PlatformUrl)
		err = d.initAppsData(kwUrl, e)
		if err != nil {
			return nil, err
		}
	}

	return d, nil
}

func (d *Deployments) initAppsData(kwUrl string, e Environment) error {
	apps, err := kube.GetAppInfos(kwUrl)
	if err != nil {
		return err
	}

	for _, app := range apps {
		if _, exists := d.Apps[app.Release]; !exists {
			d.Apps[app.Release] = &kube.AppVersions{
				AppName:  app.Release,
				Versions: make(map[string]string),
			}
		}
		d.Apps[app.Release].Versions[e.Name] = app.Version
	}

	return nil
}

// GetAppVersions returns the versions of a given app across all environments
func (d *Deployments) GetAppVersions(appName string) map[string]string {
	if app, exists := d.Apps[appName]; exists {
		return app.Versions
	}
	return nil
}

// GetAllApps returns all apps and their versions across all environments
func (d *Deployments) GetAllApps() map[string]*kube.AppVersions {
	return d.Apps
}
