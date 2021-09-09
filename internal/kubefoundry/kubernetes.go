package kubefoundry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	manifest "kubefoundry/internal/manifests"

	k8sApiMeta "k8s.io/apimachinery/pkg/api/meta"
	k8sApiMetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sApiMetaUnstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sSerializerYaml "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	k8sApiTypes "k8s.io/apimachinery/pkg/types"
	k8sClientDiscovery "k8s.io/client-go/discovery"
	k8sClientCachedMemory "k8s.io/client-go/discovery/cached/memory"
	k8sClientDynamic "k8s.io/client-go/dynamic"
	k8sClientKubernetes "k8s.io/client-go/kubernetes"
	k8sClientRestmapper "k8s.io/client-go/restmapper"
	k8sClientcmd "k8s.io/client-go/tools/clientcmd"

	//k8sUtilYaml "k8s.io/apimachinery/pkg/util/yaml"
	//k8sClientcmd "k8s.io/client-go/tools/clientcmd"
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
)

func (d *KubeFoundryCliFacade) getK8sClient() (err error) {
	config := d.c.KubeVela.KubeConfig
	if strings.HasPrefix(d.c.KubeVela.KubeConfig, "~/") {
		usr, _ := user.Current()
		dir := usr.HomeDir
		config = filepath.Join(dir, d.c.KubeVela.KubeConfig[2:])
	}
	d.kubeconfig, err = k8sClientcmd.BuildConfigFromFlags("", config)
	if err != nil {
		d.l.Errorf("Cannot read kubernetes config: %s", err.Error())
		return
	}
	d.kubeclient, err = k8sClientKubernetes.NewForConfig(d.kubeconfig)
	if err != nil {
		d.l.Errorf("Cannot connect with kubernetes cluster: %s", err.Error())
		return
	}
	return
}

func (d *KubeFoundryCliFacade) pushK8S(ctx context.Context) (err error) {
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
		d.l.Errorf("Could not read manifest: %s", err.Error())
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
		d.l.Errorf("Cannot connect to kubernetes with dynamic client: %s", err.Error())
		return err
	}
	// Decode YAML manifest into unstructured.Unstructured
	obj := &k8sApiMetaUnstructured.Unstructured{}
	_, gvk, err := decUnstructured.Decode(manifestBuff.Bytes(), nil, obj)
	if err != nil {
		d.l.Errorf("Cannot decode manifest via dynamic client: %s", err.Error())
		return err
	}
	// Find GVR kind == Application
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	fmt.Printf("group kind %s, version %s \n", gvk.GroupKind(), gvk.Version)
	fmt.Printf("mapping %s; ns %s, rs %s \n", mapping, obj.GetNamespace(), mapping.Resource)
	if err != nil {
		d.l.Errorf("Cannot find GVK: %s", err.Error())
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
		d.l.Error(err)
		return err
	}
	// Marshal object into JSON
	kubedef, err := json.Marshal(obj)
	if err != nil {
		d.l.Errorf("Cannot marshal deployment manifest into JSON: %s", err.Error())
		return err
	}
	// Create or Update the object with SSA
	//     types.ApplyPatchType indicates SSA.
	//     FieldManager specifies the field owner ID.
	_, err = dr.Patch(ctx, obj.GetName(), k8sApiTypes.ApplyPatchType, kubedef, k8sApiMetav1.PatchOptions{
		FieldManager: "kubefoundry",
	})
	if err != nil {
		d.l.Errorf("Cannot deploy application: %s", err.Error())
	}
	return err
}
