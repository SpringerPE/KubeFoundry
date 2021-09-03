package kubefoundry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	k8sApiMeta "k8s.io/apimachinery/pkg/api/meta"
	k8sApiMetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sApiMetaUnstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sSerializerYaml "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	k8sApiTypes "k8s.io/apimachinery/pkg/types"
	k8sClientDiscovery "k8s.io/client-go/discovery"
	k8sClientCachedMemory "k8s.io/client-go/discovery/cached/memory"
	k8sClientDynamic "k8s.io/client-go/dynamic"
	k8sClientKubernetes "k8s.io/client-go/kubernetes"
	k8sClientRest "k8s.io/client-go/rest"
	k8sClientRestmapper "k8s.io/client-go/restmapper"

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
	kubeconfig       *k8sClientRest.Config
	kubeclient       *k8sClientKubernetes.Clientset
	path             string
	log              log.Logger
	team             string
	c                *config.Config
	output           io.Writer
	configKubeVela   *config.KubeVela
	configDeployment *config.Deployment
	configDocker     *config.Docker
	stager           staging.AppStaging
}

func New(config *config.Config, l log.Logger) (*KubeFoundryCliFacade, error) {
	p := config.Deployment.Path
	if p == "" {
		if currentp, err := os.Getwd(); err != nil {
			p = "."
		} else {
			p = currentp
		}
	}
	d := &KubeFoundryCliFacade{
		team:             config.Team,
		path:             p,
		log:              l,
		c:                config,
		output:           os.Stdout,
		configKubeVela:   &config.KubeVela,
		configDeployment: &config.Deployment,
		configDocker:     &config.Docker,
	}
	driver := config.Deployment.StagingDriver
	l.Debugf("List of registered staging drivers: %s", staging.ListStaginDrivers())
	stager, err := staging.LoadStagingDriver(driver, config, l)
	if err != nil {
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
	// (appfile|kubefoundry|kubernetes|all)
	fullpath := d.path
	truncate := d.configDeployment.Manifest.OverWrite
	for _, man := range manifest.Types() {
		if man == manifest.CF {
			continue
		}
		if d.configDeployment.Manifest.Generate == "all" {
			fullpath = filepath.Join(d.path, man.Filename())
			err = manifest.New(man, data, fullpath, truncate, d.log)
		} else if d.configDeployment.Manifest.Generate == man.String() {
			fullpath = filepath.Join(d.path, man.Filename())
			err = manifest.New(man, data, fullpath, truncate, d.log)
			break
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
			// blocks app
			if err = app.Run(ctx, persistentVolume, env, true); err != nil {
				return err
			}
		}
	}
	return err
}

func (d *KubeFoundryCliFacade) Push(ctx context.Context) (err error) {
	// https://ymmt2005.hatenablog.com/entry/2020/04/14/An_example_of_using_dynamic_client_of_k8s.io/client-go
	var decUnstructured = k8sSerializerYaml.NewDecodingSerializer(k8sApiMetaUnstructured.UnstructuredJSONScheme)
	var dr k8sClientDynamic.ResourceInterface

	if err = d.getK8sClient(); err != nil {
		return err
	}
	manifest := filepath.Join(d.path, manifest.KubeFoundry.Filename())
	manifestBuff := bytes.NewBuffer(nil)
	if manifestfd, err := os.Open(manifest); err == nil {
		manifestBuff.ReadFrom(manifestfd)
		manifestfd.Close()
	} else {
		d.log.Errorf("Could not read manifest: %s", err.Error())
		return err
	}
	fmt.Printf("%v", manifestBuff.String())
	// Prepare a RESTMapper to find GVR
	dc, err := k8sClientDiscovery.NewDiscoveryClientForConfig(d.kubeconfig)
	if err != nil {
		return err
	}
	mapper := k8sClientRestmapper.NewDeferredDiscoveryRESTMapper(k8sClientCachedMemory.NewMemCacheClient(dc))
	//  Prepare the dynamic client
	kubeclientd, err := k8sClientDynamic.NewForConfig(d.kubeconfig)
	if err != nil {
		d.log.Errorf("Cannot connect to kubernetes with dynamic client: %s", err.Error())
		return err
	}
	// Decode YAML manifest into unstructured.Unstructured
	obj := &k8sApiMetaUnstructured.Unstructured{}
	_, gvk, err := decUnstructured.Decode(manifestBuff.Bytes(), nil, obj)
	if err != nil {
		d.log.Errorf("Cannot decode manifest via dynamic client: %s", err.Error())
		return err
	}
	// Find GVR kind == Application
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	fmt.Printf("group kind %s, version %s \n", gvk.GroupKind(), gvk.Version)
	fmt.Printf("mapping %s; ns %s, rs %s \n", mapping, obj.GetNamespace(), mapping.Resource)
	if err != nil {
		d.log.Errorf("Cannot find GVK: %s", err.Error())
		return err
	}
	// Obtain REST interface for the GVR
	if mapping.Scope.Name() == k8sApiMeta.RESTScopeNameNamespace {
		// namespaced resources should specify the namespace
		dr = kubeclientd.Resource(mapping.Resource).Namespace(obj.GetNamespace())
	} else {
		// for cluster-wide resources
		// dr = kubeclientd.Resource(mapping.Resource)
		err = fmt.Errorf("Deploying cluster-wide resources not allowed, please define your namespace")
		d.log.Error(err)
		return err
	}
	// Marshal object into JSON
	kubedef, err := json.Marshal(obj)
	if err != nil {
		d.log.Errorf("Cannot marshal deployment manifest into JSON: %s", err.Error())
		return err
	}
	// Create or Update the object with SSA
	//     types.ApplyPatchType indicates SSA.
	//     FieldManager specifies the field owner ID.
	_, err = dr.Patch(ctx, obj.GetName(), k8sApiTypes.ApplyPatchType, kubedef, k8sApiMetav1.PatchOptions{
		FieldManager: "kubefoundry",
	})
	if err != nil {
		d.log.Errorf("Cannot deploy application: %s", err.Error())
	}
	return err
}
