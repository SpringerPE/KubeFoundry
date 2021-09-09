package manifests

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	yaml "gopkg.in/yaml.v3"
)

type KVelaApplication struct {
	Name   string            `yaml:"name"`
	Image  string            `yaml:"image"`
	Env    map[string]string `yaml:"env,omitempty"`
	Routes []KVRoute         `yaml:"routes,omitempty"`
	// Other keys
}

type KVRoute struct {
	Route string `yaml:"route"`
}

type KVManifest struct {
	Path     string
	Filename string
	Name     string
	Apps     []KVelaApplication `yaml:"services"`
}

func UnmarshalKVManifest(manifest *KVManifest) (err error) {
	manifestPath := filepath.Join(manifest.Path, manifest.Filename)
	if _, err = os.Stat(manifestPath); os.IsNotExist(err) {
		err = fmt.Errorf("Cannot open KubeVela manifest '%s': %s", manifestPath, err.Error())
		return
	}
	data, errPath := ioutil.ReadFile(manifestPath)
	if errPath != nil {
		err = fmt.Errorf("Failed to read manifest '%s': %s", manifestPath, errPath.Error())
		return
	}
	err = yaml.Unmarshal(data, manifest)
	if err != nil || manifest == nil {
		if err == nil {
			err = fmt.Errorf("empty manifest")
		}
		err = fmt.Errorf("Failed to unmarshall manifest %s: %s", manifestPath, err.Error())
		return
	}
	return nil
}

func ParseKVManifest(filename string) (manifest *KVManifest, err error) {
	manifest = &KVManifest{}
	manifest.Path = filepath.Dir(filename)
	manifest.Filename = filepath.Base(filename)
	err = UnmarshalKVManifest(manifest)
	return
}

func (m *KVManifest) Applications() (apps []string) {
	for _, app := range m.Apps {
		apps = append(apps, app.Name)
	}
	return
}

func (m *KVManifest) GetApplication(name string) (app *KVelaApplication, err error) {
	for _, app := range m.Apps {
		if app.Name == name {
			return &app, nil
		}
	}
	err = fmt.Errorf("Application '%s' not found in manifest", name)
	return nil, err
}
