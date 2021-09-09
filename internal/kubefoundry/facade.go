package kubefoundry

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	k8sClientKubernetes "k8s.io/client-go/kubernetes"
	k8sClientRest "k8s.io/client-go/rest"

	//k8sUtilYaml "k8s.io/apimachinery/pkg/util/yaml"
	//k8sClientcmd "k8s.io/client-go/tools/clientcmd"
	// Uncomment to load all auth plugins
	//_ "k8s.io/client-go/plugin/pkg/client/auth"

	config "kubefoundry/internal/config"
	log "kubefoundry/internal/log"
	manifest "kubefoundry/internal/manifests"
	staging "kubefoundry/internal/staging"

	// Register Staging drivers
	_ "kubefoundry/internal/staging/dockerstaging"
)

type KubeFoundryCliFacade struct {
	kubeconfig *k8sClientRest.Config
	kubeclient *k8sClientKubernetes.Clientset
	path       string
	l          log.Logger
	team       string
	c          *config.Config
	output     io.Writer
	stager     staging.AppStaging
}

func New(config *config.Config, l log.Logger) (*KubeFoundryCliFacade, error) {
	p := config.Deployment.Path
	if p == "" {
		p = "."
		if currentp, err := os.Getwd(); err == nil {
			p = currentp
		}
	}
	d := &KubeFoundryCliFacade{
		team:   config.Team,
		path:   p,
		l:      l,
		c:      config,
		output: os.Stdout,
	}
	driver := config.Deployment.StagingDriver
	d.l.Debugf("List of registered staging drivers: %s", staging.ListStaginDrivers())
	stager, err := staging.LoadStagingDriver(driver, config, l)
	if err != nil {
		d.l.Error(err)
		return nil, err
	}
	d.stager = stager
	return d, nil
}

func (d *KubeFoundryCliFacade) GenerateManifest() (err error) {
	data, err := d.getMetadata()
	if err != nil {
		return err
	}
	//  d.c.Deployment.Manifest.Generate
	// (appfile|kubefoundry|kubernetes|all)
	fullpath := d.path
	truncate := d.c.Deployment.Manifest.OverWrite
	for _, man := range manifest.Types() {
		if man == manifest.CF || man == manifest.Unknown {
			continue
		}
		fullpath = filepath.Join(d.path, man.Filename())
		if d.c.Deployment.Manifest.Generate == "all" || d.c.Deployment.Manifest.Generate == man.String() {
			d.l.Infof("Generating %s manifest: %s", man.String(), fullpath)
			err = manifest.New(man, data, fullpath, truncate)
			if err != nil {
				d.l.Errorf("Unable to generate %s manifest: %s", man.String(), err.Error())
				break
			}
		}
	}
	return err
}

func (d *KubeFoundryCliFacade) StageApp(ctx context.Context, build, push bool) (err error) {
	apps, err := d.initStager()
	if err == nil {
		for _, app := range apps {
			if build {
				if _, err := app.Build(ctx); err != nil {
					return err
				}
			}
			if push {
				if err = app.Push(ctx); err != nil {
					return err
				}
			}
		}
	}
	return err
}

func (d *KubeFoundryCliFacade) RunApp(ctx context.Context, persistentVolume string, env map[string]string) (err error) {
	apps, err := d.initStager()
	if err == nil {
		for _, app := range apps {
			if err = app.Run(ctx, persistentVolume, env, true); err != nil {
				return err
			}
		}
	}
	return err
}

func (d *KubeFoundryCliFacade) Push(ctx context.Context) (err error) {
	manifest := filepath.Join(d.path, manifest.KubeFoundry.Filename())
	manifestBuff := bytes.NewBuffer(nil)
	if manifestfd, err := os.Open(manifest); err == nil {
		manifestBuff.ReadFrom(manifestfd)
		manifestfd.Close()
	} else {
		d.l.Errorf("Could not read manifest: %s", err.Error())
		return err
	}
	fmt.Printf("%v", manifestBuff.String())
	return err
}
