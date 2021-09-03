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
	K8S
)

func Types() []ManifestType {
	return []ManifestType{Unknown, CF, AppFile, KubeFoundry, K8S}
}

func Type(text string) (m ManifestType) {
	switch strings.ToLower(text) {
	case "cf":
		return CF
	case "appfile":
		return AppFile
	case "kubefoundry":
		return KubeFoundry
	case "kubernetes":
		return K8S
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
		return "app.yml"
	case K8S:
		return "deploy.yml"
	default:
		return ""
	}
}

func (m ManifestType) String() string {
	kinds := [...]string{"Unknown", "CF", "AppFile", "KubeFoundry", "K8S"}
	return kinds[int(m)]
}

type Generator struct {
	output    io.Writer
	templates *template.Template
	log       log.Logger
}

func NewGenerator(output io.Writer, l log.Logger) (*Generator, error) {
	templates, err := template.ParseFS(EmbedK8sTemplates, "templates/*")
	if err != nil {
		panic(err)
	}
	m := &Generator{
		templates: templates,
		output:    output,
		log:       l,
	}
	return m, nil
}

func (m *Generator) Generate(kind ManifestType, data *ContextData) (err error) {
	if kind == CF {
		err = fmt.Errorf("Cannot generate CF manifest file")
	} else {
		if filename := kind.Filename(); filename != "" {
			err = m.templates.ExecuteTemplate(m.output, filename+".tmpl", data)
		} else {
			err = fmt.Errorf("Unknown manifest type")
		}
	}
	if err != nil {
		m.log.Error(err)
	}
	return
}

func New(kind ManifestType, data *ContextData, fullpath string, truncate bool, l log.Logger) error {
	flags := os.O_RDWR | os.O_CREATE
	if truncate {
		flags = os.O_RDWR | os.O_CREATE | os.O_TRUNC
	}
	target, err := os.OpenFile(fullpath, flags, 0755)
	if err != nil {
		err = fmt.Errorf("Unable to create manifest: %s", err.Error())
		return err
	}
	defer target.Close()
	m, err := NewGenerator(target, l)
	if err != nil {
		return err
	}
	l.Infof("Generating %s manifest '%s' ...", kind.String(), fullpath)
	return m.Generate(kind, data)
}
