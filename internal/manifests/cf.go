package manifests

import (
	"crypto/md5"
	"fmt"
	"hash"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

type CfApplication struct {
	Name        string            `yaml:"name"`
	Buildpack   string            `yaml:"buildpack,omitempty"`
	Buildpacks  []string          `yaml:"buildpacks,omitempty"`
	Env         map[string]string `yaml:"env,omitempty"`
	NoRoute     bool              `yaml:"no-route,omitempty"`
	RandomRoute bool              `yaml:"random-route,omitempty"`
	Routes      []CfRoute         `yaml:"routes,omitempty"`
	Memory      string            `yaml:"memory,omitempty"`
	Path        string            `yaml:"path,omitempty"`
	Instances   int               `yaml:"instances,omitempty"`
	// Other keys
}

type CfRoute struct {
	Route string `yaml:"route"`
}

type CfManifest struct {
	Path     string
	Filename string
	Apps     []CfApplication `yaml:"applications"`
}

func UnmarshalCfManifest(manifest *CfManifest) (err error) {
	manifestPath := filepath.Join(manifest.Path, manifest.Filename)
	if _, err = os.Stat(manifestPath); os.IsNotExist(err) {
		err = fmt.Errorf("Cannot open CF manifest '%s': %s", manifestPath, err.Error())
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

func ParseCfManifest(filename string) (manifest *CfManifest, err error) {
	manifest = &CfManifest{}
	manifest.Path = filepath.Dir(filename)
	manifest.Filename = filepath.Base(filename)
	err = UnmarshalCfManifest(manifest)
	return
}

func (m *CfManifest) Applications() (apps []string) {
	for _, app := range m.Apps {
		apps = append(apps, app.Name)
	}
	return
}

func (m *CfManifest) GetApplication(name string) (app *CfApplication, err error) {
	for _, app := range m.Apps {
		if app.Name == name {
			return &app, nil
		}
	}
	err = fmt.Errorf("Application '%s' not found in manifest", name)
	return nil, err
}

func (app *CfApplication) GetBuildpacks() (bps []string, err error) {
	if app.Buildpack != "" {
		bps = append(bps, app.Buildpack)
	} else if len(app.Buildpacks) > 0 {
		bps = app.Buildpacks
	}
	return
}

// https://github.com/google/uuid/blob/master/hash.go
func (app *CfApplication) uuid(h hash.Hash, name, data []byte, version int) (uuid [16]byte) {
	h.Reset()
	h.Write(name) //nolint:errcheck
	h.Write(data) //nolint:errcheck
	s := h.Sum(nil)
	copy(uuid[:], s)
	uuid[6] = (uuid[6] & 0x0f) | uint8((version&0xf)<<4)
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // RFC 4122 variant
	return
}

func (app *CfApplication) GetUUID(data string) (uuid string) {
	u := app.uuid(md5.New(), []byte(app.Name), []byte(data), 3)
	uuid = fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:])
	return
}

// Parses the human-readable size string into the amount it represents.
func (app *CfApplication) ParseSize() (int64, error) {
	if app.Memory == "" {
		// 1G by default
		return int64(1073741824), nil
	}
	sizeRegex := regexp.MustCompile(`^(\d+(\.\d+)*) ?([kKmMgGtTpP])?[iI]?[bB]?$`)
	calcMap := map[string]int64{
		"k": 1024,
		"m": 1048576,
		"g": 1073741824,
	}
	matches := sizeRegex.FindStringSubmatch(app.Memory)
	if len(matches) != 4 {
		return -1, fmt.Errorf("Invalid size defined in manifest: '%s'", app.Memory)
	}
	size, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return -1, fmt.Errorf("Invalid size defined in manifest: %s", err.Error())
	}
	unitPrefix := strings.ToLower(matches[3])
	if mul, ok := calcMap[unitPrefix]; ok {
		size *= float64(mul)
	}
	return int64(size), nil
}

func (app *CfApplication) GetResources(cpuMemoryFactor float64) (memory int64, cpu float64, err error) {
	cpu = float64(1)
	memory, err = app.ParseSize()
	if err == nil {
		if cpuMemoryFactor > 0.1 {
			cpu = float64(memory) / cpuMemoryFactor
		} else {
			cpu = float64(1)
		}
	} else {
		// 1G
		memory = int64(1073741824)
	}
	return
}

func (app *CfApplication) GetRoutes(randomDomain string) (routes []string, err error) {
	if app.RandomRoute {
		if r := app.GetUUID(randomDomain); r != "" {
			routes = append(routes, r+"."+randomDomain)
		} else {
			err = fmt.Errorf("Cannot get UUID for application '%s'", app.Name)
		}
	} else if len(app.Routes) > 0 {
		for _, r := range app.Routes {
			routes = append(routes, r.Route)
		}
	}
	return
}
