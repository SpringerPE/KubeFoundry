package kubefoundry

import (
	"os/user"
	"path/filepath"
	"strings"

	k8sClientKubernetes "k8s.io/client-go/kubernetes"
	k8sClientcmd "k8s.io/client-go/tools/clientcmd"
	//k8sApiMeta "k8s.io/apimachinery/pkg/api/meta"
	//k8sApiMetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//k8sUtilYaml "k8s.io/apimachinery/pkg/util/yaml"
	//k8sApiMetaUnstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	//k8sSerializerYaml "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	//k8sApiTypes "k8s.io/apimachinery/pkg/types"
	//k8sClientDiscovery "k8s.io/client-go/discovery"
	//k8sClientCachedMemory "k8s.io/client-go/discovery/cached/memory"
	//k8sClientDynamic "k8s.io/client-go/dynamic"
	//k8sClientRest "k8s.io/client-go/rest"
	//k8sClientRestmapper "k8s.io/client-go/restmapper"
	// Uncomment to load all auth plugins
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	manifest "kubefoundry/internal/manifests"
	staging "kubefoundry/internal/staging"
)

func (d *KubeFoundryCliFacade) getMetadata() (data *manifest.ContextData, err error) {
	appName := d.configDeployment.AppName
	appVersion := d.configDeployment.AppVersion
	appPort := d.configDeployment.Defaults.Port
	rs := &manifest.ResourceData{
		Domain: d.configDeployment.Defaults.Domain,
		CPU:    d.configDeployment.Defaults.CPU,
		Mem:    d.configDeployment.Defaults.Mem,
		Disk:   d.configDeployment.Defaults.Disk,
	}
	kube := &manifest.KubeData{
		NameSpace:   d.configKubeVela.Namespace,
		Environment: d.configKubeVela.Environment,
		Api:         d.configKubeVela.Api,
		Cluster:     d.configKubeVela.Cluster,
	}
	data = manifest.NewContextData(d.path, d.team, d.configDeployment.RegistryTag, d.configDeployment.Args, kube, nil, d.log)
	// (try|yes|no)
	if d.configDeployment.Manifest.ParseCF == "try" || d.configDeployment.Manifest.ParseCF == "yes" {
		err = data.GetContextDataApp(appName, appVersion, appPort, true, rs)
		if err != nil {
			if d.configDeployment.Manifest.ParseCF == "yes" {
				d.log.Error(err)
				return nil, err
			} else {
				d.log.Warn(err.Error())
				err = data.GetContextDataApp(appName, appVersion, appPort, false, rs)
			}
		}
	} else {
		err = data.GetContextDataApp(appName, appVersion, appPort, false, rs)
	}
	if err != nil {
		d.log.Error(err)
	}
	return data, err
}

func (d *KubeFoundryCliFacade) initStager() ([]staging.AppPackage, error) {
	// TODO: Load manifest!!!
	data, err := d.getMetadata()
	if err == nil {
		return d.stager.Stager(data, d.output)
	}
	return nil, err
}

func (d *KubeFoundryCliFacade) getK8sClient() (err error) {
	config := d.configKubeVela.KubeConfig
	if strings.HasPrefix(d.configKubeVela.KubeConfig, "~/") {
		usr, _ := user.Current()
		dir := usr.HomeDir
		config = filepath.Join(dir, d.configKubeVela.KubeConfig[2:])
	}
	d.kubeconfig, err = k8sClientcmd.BuildConfigFromFlags("", config)
	if err != nil {
		d.log.Errorf("Cannot read kubernetes config: %s", err.Error())
		return
	}
	d.kubeclient, err = k8sClientKubernetes.NewForConfig(d.kubeconfig)
	if err != nil {
		d.log.Errorf("Cannot connect with kubernetes cluster: %s", err.Error())
		return
	}
	return
}
