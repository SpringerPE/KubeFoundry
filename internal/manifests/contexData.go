package manifests

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	log "kubefoundry/internal/log"

	gitrepo "github.com/go-git/go-git/v5"
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
	log       log.Logger
}

func NewContextData(contextDir, team, imgRegistry string, args map[string]string, kube *KubeData, cf *CfData, l log.Logger) *ContextData {
	t := time.Now().UTC()
	ref := t.Format("20060102150405")
	// check if path is a git repo and get commit
	opts := gitrepo.PlainOpenOptions{
		DetectDotGit: true,
	}
	git := ""
	if repo, err := gitrepo.PlainOpenWithOptions(contextDir, &opts); err == nil {
		if head, err := repo.Head(); err == nil {
			// 20 first chars from hash
			ref = head.Strings()[1][1:21]
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
		log:       l,
	}
	return contextData
}

func (d *ContextData) GetContextDataApp(name, version string, port int, parseCfManifest bool, rs *ResourceData) (err error) {
	var apps []*AppData

	if parseCfManifest {
		apps, err = d.GetContextDataCF(name, version, port, rs)
		if err != nil {
			return
		}
	} else {
		apps = d.GetContextDataDefault(name, version, port, rs)
	}
	d.Apps = append(d.Apps, apps...)
	return nil
}

func (d *ContextData) GetContextDataDefault(name, version string, port int, rs *ResourceData) (apps []*AppData) {
	appname := name
	if appname == "" {
		// no concurrency here!
		appname = fmt.Sprintf("%s-webapp-%d", d.Name, len(d.Apps))
	}
	image := appname + ":" + d.Ref
	appversion := "latest"
	hostname := appname + "-" + d.Ref
	routes := make(map[string]string)
	if version != "" {
		image = appname + ":" + version
		appversion = version
		hostname = appname + "-" + version
	}
	if d.Registry != "" {
		image = d.Registry + "/" + d.Team + "/" + image
	}
	if rs != nil && rs.Domain != "" {
		routes["http-0"] = strings.ToLower(hostname + "." + rs.Domain)
	}
	appData := AppData{
		Name:      appname,
		Dir:       d.Dir,
		Image:     image,
		Version:   appversion,
		Routes:    routes,
		Env:       make(map[string]string),
		Instances: 1,
		Port:      port,
		Resources: rs,
	}
	apps = append(apps, &appData)
	return
}

func (d *ContextData) GetContextDataCF(name, version string, port int, rs *ResourceData) (apps []*AppData, err error) {
	if d.CF == nil {
		d.CF = &CfData{}
	}
	if cfm, errm := ParseCfManifest(d.Dir, d.log); errm != nil {
		return apps, errm
	} else {
		d.CF.Manifest = cfm
		for _, app := range cfm.Applications() {
			if name != "" && app != name {
				continue
			}
			appManifest, _ := cfm.GetApplication(app)
			image := app + ":" + d.Ref
			appversion := "latest"
			routes := make(map[string]string)
			if rs != nil {
				if rs.Domain != "" {
					if cfoutes, err := appManifest.GetRoutes(rs.Domain); err == nil {
						for i, rt := range cfoutes {
							routes["http-"+strconv.Itoa(i)] = rt
						}
					} else {
						if version != "" {
							routes["http-0"] = app + "-" + version + "." + rs.Domain
						} else {
							routes["http-0"] = app + "-" + d.Ref + "." + rs.Domain
						}
					}
				}
			} else {
				rs = &ResourceData{}
			}
			if version != "" {
				appversion = version
				image = app + ":" + version
			}
			if d.Registry != "" {
				image = d.Registry + "/" + d.Team + "/" + image
			}
			if mem, cpu, err := appManifest.GetResources(0.0); err == nil {
				rs.CPU = strconv.FormatFloat(cpu, 'f', -1, 64)
				rs.Mem = strconv.FormatInt(mem, 10)
			}
			appData := AppData{
				Name:      app,
				Dir:       d.Dir,
				Image:     image,
				Version:   appversion,
				Routes:    routes,
				Env:       appManifest.Env,
				Instances: 1,
				Port:      port,
				Resources: rs,
			}
			apps = append(apps, &appData)
		}
	}
	return
}
