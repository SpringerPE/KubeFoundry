package staging

import (
	"context"
	"io"

	config "kubefoundry/internal/config"
	log "kubefoundry/internal/log"
	manifest "kubefoundry/internal/manifests"
)

type AppStaging interface {
	New(c *config.Config, l log.Logger) (AppStaging, error)
	Stager(data *manifest.ContextData, output io.Writer) ([]AppPackage, error)
	Finish(ctx context.Context, appPackages []AppPackage) error
}

type AppPackage interface {
	Build(ctx context.Context) (string, error)
	Info(ctx context.Context) (map[string]interface{}, error)
	Push(ctx context.Context) error
	Run(ctx context.Context, dataDir string, env map[string]string, output bool) error
	Destroy(ctx context.Context, all bool) (err error)
}
