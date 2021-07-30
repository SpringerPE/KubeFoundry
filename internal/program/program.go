package program

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"

	"kubefoundry/internal/config"
	"kubefoundry/internal/config/configurator"
	"kubefoundry/internal/kubefoundry"

	cobra "github.com/spf13/cobra"
)

type Program struct {
	Build        string
	Config       *config.Config
	Data         interface{}
	ConfigArg    string
	Configurator configurator.Configurator
}

func NewProgram(build, version, configArg string, command *cobra.Command) *Program {
	p := Program{
		Build:        build,
		ConfigArg:    configArg,
		Configurator: configurator.New(version, configArg, command),
	}
	return &p
}

func (p *Program) Init() {
	p.Config = p.Configurator.InitConfig()
}

func (p *Program) LoadConfig() error {
	cfg, err := p.Configurator.LoadConfig(p.ConfigArg)
	if err == nil {
		log := p.Configurator.Logger()
		f := p.Configurator.GetConfigFile(false)
		log.Infof("Configuration loaded from file: %s", f)
		if err = p.Configurator.CheckConfig(cfg); err == nil {
			p.Config = cfg
		}
		return err
	}
	return err
}

func (p *Program) GetJsonConfig() ([]byte, error) {
	cfg, err := p.Configurator.GetConfigMap(p.Config)
	if err == nil {
		return json.MarshalIndent(cfg, "", "  ")
	}
	return []byte{}, err
}

func (p *Program) GenerateManifest() (err error) {
	log := p.Configurator.Logger()
	if action, err := kubefoundry.New(p.Config, log); err == nil {
		return action.GenerateManifest()
	}
	return nil
}

func (p *Program) BuildAppImage() (err error) {
	log := p.Configurator.Logger()
	if action, err := kubefoundry.New(p.Config, log); err == nil {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		return action.StageApp(ctx, true, false)
	}
	return nil
}

func (p *Program) StageAppImage() (err error) {
	log := p.Configurator.Logger()
	if action, err := kubefoundry.New(p.Config, log); err == nil {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		return action.StageApp(ctx, true, true)
	}
	return nil
}

func (p *Program) UploadAppImage() (err error) {
	log := p.Configurator.Logger()
	if action, err := kubefoundry.New(p.Config, log); err == nil {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		return action.StageApp(ctx, false, true)
	}
	return nil
}

func (p *Program) RunAppImage(env map[string]string) (err error) {
	log := p.Configurator.Logger()
	if action, err := kubefoundry.New(p.Config, log); err == nil {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		persistentvol := ""
		return action.RunApp(ctx, persistentvol, env)
	}
	return nil
}

func (p *Program) PushApp() (err error) {
	log := p.Configurator.Logger()
	if action, err := kubefoundry.New(p.Config, log); err == nil {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		return action.Push(ctx)
	}
	return nil
}
