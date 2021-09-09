package kubefoundry

import (
	"path/filepath"
	"strconv"

	manifest "kubefoundry/internal/manifests"
	staging "kubefoundry/internal/staging"
)

func (d *KubeFoundryCliFacade) getMetadata() (data *manifest.ContextData, err error) {
	appName := d.c.Deployment.AppName
	appVersion := d.c.Deployment.AppVersion
	appPath := d.c.Deployment.AppPath
	appRoutes := make(map[string]string)
	for i, r := range d.c.Deployment.AppRoutes {
		// format is  route[<PORT>-<INDEX>] = r
		appRoutes[strconv.Itoa(d.c.Deployment.Defaults.Port)+"-"+strconv.Itoa(i)] = r
	}
	rs := &manifest.ResourceData{
		Domain: d.c.Deployment.Defaults.Domain,
		Port:   d.c.Deployment.Defaults.Port,
		CPU:    d.c.Deployment.Defaults.CPU,
		Mem:    d.c.Deployment.Defaults.Mem,
		Disk:   d.c.Deployment.Defaults.Disk,
	}
	kube := &manifest.KubeData{
		NameSpace:   d.c.KubeVela.Namespace,
		Environment: d.c.KubeVela.Environment,
		Api:         d.c.KubeVela.Api,
		Cluster:     d.c.KubeVela.Cluster,
	}
	cf := &manifest.CfData{
		Space: d.c.CF.Space,
		Org:   d.c.CF.Org,
		Api:   d.c.CF.API,
	}
	manifestPath := filepath.Dir(d.c.CF.Manifest)
	manifestFile := filepath.Base(d.c.CF.Manifest)
	if manifestPath == "." {
		manifestPath = d.path
	}
	cf.Manifest = &manifest.CfManifest{
		Path:     manifestPath,
		Filename: manifestFile,
		Apps:     []manifest.CfApplication{},
	}
	data = manifest.NewContextMetadata(d.path, d.team, d.c.Deployment.RegistryTag, d.c.Deployment.Args, kube, cf)
	// (try|yes|no)
	if d.c.CF.ReadManifest != "no" {
		if err = data.GetAppContextMetadata(appPath, appName, appVersion, appRoutes, rs, true); err != nil {
			if d.c.CF.ReadManifest == "yes" {
				d.l.Error(err)
				return nil, err
			}
			d.l.Warn(err.Error())
		} else {
			return data, err
		}
	}
	if err = data.GetAppContextMetadata(appPath, appName, appVersion, appRoutes, rs, false); err != nil {
		d.l.Error(err)
	}
	return data, err
}

func (d *KubeFoundryCliFacade) initStager() ([]staging.AppPackage, error) {
	data, err := d.getMetadata()
	if err != nil {
		return nil, err
	}
	return d.stager.Stager(data, d.output)
}
