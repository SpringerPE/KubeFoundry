package manifests

import (
	"embed"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"

	log "kubefoundry/internal/log"
)

// EmbedK8sTemplates holds static files
//go:embed templates/*.tmpl
var EmbedK8sTemplates embed.FS

type ManifestType int

const (
	Unknown ManifestType = iota // 0
	CF
	AppFile
	KubeFoundry
)

func GetManifestType(text string) (m ManifestType) {
	switch strings.ToLower(text) {
	case "cf":
		return CF
	case "appfile":
		return AppFile
	case "kubefoundry":
		return KubeFoundry
	case "kubernetes":
		return KubeFoundry
	default:
		return Unknown
	}
}

func (m ManifestType) Filename() string {
	switch m {
	case CF:
		return "manifest.yml"
	case AppFile:
		return "vela.yml"
	case KubeFoundry:
		return "deploy.yml"
	default:
		return ""
	}
}

func (m ManifestType) String() string {
	kinds := [...]string{"Unknown", "CF", "AppFile", "KubeFoundry"}
	return kinds[int(m)]
}

type ManifestGenerator struct {
	output    io.Writer
	templates *template.Template
	log       log.Logger
}

func NewManifestGenerator(output io.Writer, l log.Logger) (*ManifestGenerator, error) {
	templates, err := template.ParseFS(EmbedK8sTemplates, "templates/*")
	if err != nil {
		panic(err)
	}
	m := &ManifestGenerator{
		templates: templates,
		output:    output,
		log:       l,
	}
	return m, nil
}

func (m *ManifestGenerator) Generate(kind ManifestType, data *ContextData) (err error) {
	switch kind {
	case CF:
		err = fmt.Errorf("Generate CF manifest file not implemented")
	case AppFile:
		err = m.templates.ExecuteTemplate(m.output, "vela.yml.tmpl", data)
	case KubeFoundry:
		err = m.templates.ExecuteTemplate(m.output, "k8s.yml.tmpl", data)
	default:
		err = fmt.Errorf("Unknown manifest type")
	}
	if err != nil {
		m.log.Error(err)
	}
	return
}

func NewManifestFile(kind ManifestType, data *ContextData, fullpath string, truncate bool, l log.Logger) error {
	flags := os.O_RDWR | os.O_CREATE
	if truncate {
		flags = os.O_RDWR | os.O_CREATE | os.O_TRUNC
	}
	target, err := os.OpenFile(fullpath, flags, 0755)
	if err != nil {
		err = fmt.Errorf("Unable to create manifest file: %s", err.Error())
		return err
	}
	defer target.Close()
	m, err := NewManifestGenerator(target, l)
	if err != nil {
		return err
	}
	l.Infof("Generating %s manifest '%s' ...", kind.String(), fullpath)
	return m.Generate(kind, data)
}
