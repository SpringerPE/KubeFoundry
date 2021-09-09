package manifests

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	gitrepo "github.com/go-git/go-git/v5"
)

var (
	DefaultDomain     string = "local"
	DefaultPort       int    = 8080
	DefaultCPU        string = "1"
	DefaultMem        string = "1024M"
	DefaultDisk       string = "4G"
	DefaultRefVersion string = "latest"
)

type CfData struct {
	Space    string
	Org      string
	Api      string
	Manifest *CfManifest
}

type KubeData struct {
	NameSpace   string
	Environment string
	Api         string
	Cluster     string
}

type ResourceData struct {
	Domain string
	Port   int
	CPU    string
	Mem    string
	Disk   string
}

type AppData struct {
	Name      string
	Dir       string
	Image     string
	Version   string
	Routes    map[string]string
	Env       map[string]string
	Instances int
	Port      int
	Resources *ResourceData
}

type ContextData struct {
	Dir       string
	Git       string
	Name      string
	Date      time.Time
	DateHuman string
	Registry  string
	Ref       string
	Team      string
	Env       map[string]string
	Args      map[string]string
	Apps      []*AppData
	Kubevela  *KubeData
	CF        *CfData
}

func NewDefaultResourceData() *ResourceData {
	rs := &ResourceData{
		Domain: DefaultDomain,
		Port:   DefaultPort,
		CPU:    DefaultCPU,
		Mem:    DefaultMem,
		Disk:   DefaultDisk,
	}
	return rs
}

func NewContextMetadata(contextDir, team, imgRegistry string, args map[string]string, kube *KubeData, cf *CfData) *ContextData {
	t := time.Now().UTC()
	//ref := t.Format("20060102150405")
	ref := DefaultRefVersion
	// check if path is a git repo and get commit
	opts := gitrepo.PlainOpenOptions{DetectDotGit: true}
	git := ""
	if repo, err := gitrepo.PlainOpenWithOptions(contextDir, &opts); err == nil {
		if head, err := repo.Head(); err == nil {
			// 13 first chars from hash
			ref = head.Strings()[1][1:13]
		}
		if remotes, err := repo.Remotes(); err == nil {
			git = remotes[0].Config().URLs[0]
		}
	}
	env := make(map[string]string)
	for _, setting := range os.Environ() {
		pair := strings.SplitN(setting, "=", 2)
		env[pair[0]] = pair[1]
	}
	contextData := &ContextData{
		Dir:       contextDir,
		Git:       git,
		Name:      filepath.Base(contextDir),
		Date:      t,
		DateHuman: t.String(),
		Registry:  imgRegistry,
		Ref:       ref,
		Team:      team,
		Args:      args,
		Env:       env,
		Apps:      []*AppData{},
		Kubevela:  kube,
		CF:        cf,
	}
	return contextData
}

func (d *ContextData) GetAppContextMetadata(dir, name, version string, routes map[string]string, rs *ResourceData, parseCF bool) (err error) {
	var apps []*AppData
	if rs == nil {
		rs = NewDefaultResourceData()
	}
	if dir == "" {
		dir = d.Dir
	}
	if parseCF {
		if d.CF != nil && d.CF.Manifest != nil && d.CF.Manifest.Filename != "" {
			apps, err = d.getAppContextMetadataCF(dir, name, version, routes, rs)
			if err != nil {
				return
			}
		} else {
			return fmt.Errorf("Unable to get CloudFoundry metadata, settings not defined")
		}
	} else {
		apps = d.getAppContextMetadataDefault(dir, name, version, routes, rs)
	}
	// TODO: Load appfile and merge?
	d.Apps = append(d.Apps, apps...)
	return nil
}

func (d *ContextData) getAppContextMetadataDefault(dir, name, version string, routes map[string]string, rs *ResourceData) (apps []*AppData) {
	if name == "" {
		name = fmt.Sprintf("%s-webapp-%d", d.Name, len(d.Apps))
	}
	if version == "" {
		version = d.Ref
	}
	image := name + ":" + version
	if d.Registry != "" {
		image = d.Registry + "/" + d.Team + "/" + image
	}
	appRoutes := make(map[string]string)
	for k, v := range routes {
		appRoutes[k] = v
	}
	if len(appRoutes) == 0 && rs.Domain != "" {
		hostname := name + "-" + version
		appRoutes[strconv.Itoa(rs.Port)+"-0"] = strings.ToLower(hostname + "." + rs.Domain)
	}
	appData := AppData{
		Name:      name,
		Dir:       dir,
		Image:     image,
		Version:   version,
		Routes:    appRoutes,
		Env:       make(map[string]string),
		Instances: 1,
		Port:      rs.Port,
		Resources: rs,
	}
	apps = append(apps, &appData)
	return
}

func (d *ContextData) getAppContextMetadataCF(dir, name, version string, routes map[string]string, rs *ResourceData) (apps []*AppData, err error) {
	err = UnmarshalCfManifest(d.CF.Manifest)
	if err != nil {
		return
	} else {
		for _, app := range d.CF.Manifest.Applications() {
			if name != "" && app != name {
				// Select only the application name
				continue
			}
			appManifest, _ := d.CF.Manifest.GetApplication(app)
			name := app
			if version == "" {
				version = d.Ref
			}
			image := name + ":" + version
			if d.Registry != "" {
				image = d.Registry + "/" + d.Team + "/" + image
			}
			appRoutes := make(map[string]string)
			for k, v := range routes {
				appRoutes[k] = v
			}
			if len(appRoutes) == 0 && rs.Domain != "" {
				if cfoutes, err := appManifest.GetRoutes(rs.Domain); err == nil {
					for i, rt := range cfoutes {
						appRoutes[strconv.Itoa(rs.Port)+"-"+strconv.Itoa(i)] = rt
					}
				} else {
					hostname := name + "-" + version
					appRoutes[strconv.Itoa(rs.Port)+"-0"] = strings.ToLower(hostname + "." + rs.Domain)
				}
			}
			instances := 1
			if appManifest.Instances > 0 {
				instances = appManifest.Instances
			}
			path := dir
			if appManifest.Path != "" {
				path = appManifest.Path
			}
			if mem, cpu, err := appManifest.GetResources(0.0); err == nil {
				rs.CPU = strconv.FormatFloat(cpu, 'f', -1, 64)
				rs.Mem = strconv.FormatInt(mem, 10)
			}
			appData := AppData{
				Name:      app,
				Dir:       path,
				Image:     image,
				Version:   version,
				Routes:    appRoutes,
				Env:       appManifest.Env,
				Instances: instances,
				Port:      rs.Port,
				Resources: rs,
			}
			apps = append(apps, &appData)
		}
	}
	return
}
