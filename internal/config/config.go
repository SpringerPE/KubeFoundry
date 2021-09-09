package config

const (
	ConfigType      string = "yaml"
	ConfigFile      string = "config.yml"
	ConfigPath      string = "~/.kubefoundry"
	ConfigEnv       string = "KF"
	ConfigUserAgent string = "kubefoundry"
)

type KubeVela struct {
	Api         string `mapstructure:"api"`
	KubeConfig  string `mapstructure:"kubeconfig" valid:"required" default:"~/.kube/config" flag:"kubernetes config"`
	Cluster     string `mapstructure:"cluster"`
	Environment string `mapstructure:"environment" valid:"required" flag:"kubevela environment"`
	Namespace   string `mapstructure:"namespace" valid:"required" flag:"kubernetes namespace"`
}

type Docker struct {
	API      string `mapstructure:"api" flag:"docker api"`
	Registry string `mapstructure:"registry"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type CF struct {
	API          string `mapstructure:"api" flag:"cf api"`
	Org          string `mapstructure:"org" flag:"cf org"`
	Space        string `mapstructure:"space" flag:"cf space"`
	Manifest     string `mapstructure:"manifest" valid:"required" default:"manifest.yml" flag:"cf manifest"`
	ReadManifest string `mapstructure:"readmanifest" valid:"in(yes|no|try),required" default:"try" flag:"cf read manifest"`
}

type Defaults struct {
	Domain string `mapstructure:"domain" valid:"required" flag:"default domain"`
	Port   int    `mapstructure:"port" default:"8080" flag:"default port"`
	Mem    string `mapstructure:"mem" default:"1G" flag:"default mem"`
	CPU    string `mapstructure:"cpu" default:"1" flag:"default cpu"`
	Disk   string `mapstructure:"disk" default:"4G" flag:"default disk"`
}

type Manifest struct {
	AppFile   string `mapstructure:"appfile" default:"vela.yml" valid:"required" flag:"kubevela appfile"`
	Generate  string `mapstructure:"generate" valid:"in(appfile|kubefoundry|kubernetes|all),required" default:"kubefoundry" flag:"manifest generate"`
	OverWrite bool   `mapstructure:"overwrite" default:"true" flag:"manifest overwrite"`
}

type Deployment struct {
	Path          string            `mapstructure:"path" default:"" flag:"path"`
	AppPath       string            `mapstructure:"apppath" default:"" flag:"app path"`
	AppName       string            `mapstructure:"appname" default:"" flag:"app"`
	AppVersion    string            `mapstructure:"appversion" default:"" flag:"version"`
	AppRoutes     []string          `mapstructure:"approutes" default:"[]" flag:"routes"`
	StagingDriver string            `mapstructure:"stagingdriver" valid:"required" default:"DockerStaging" flag:"staging"`
	Args          map[string]string `mapstructure:"args" flag:"args"`
	RegistryTag   string            `mapstructure:"registry" flag:"registry prefix"`
	Defaults      Defaults          `mapstructure:"defaults"`
	Manifest      Manifest          `mapstructure:"manifest"`
}

// This config what the driver gets (dockerstaging)
type DockerStaging struct {
	RemoveBeforeBuild bool   `mapstructure:"removebeforebuild" default:"true"`
	RestartPolicy     string `mapstructure:"restartpolicy" valid:"in(no|unless-stopped|on-failure)" default:"unless-stopped"`
	DynamicPorts      bool   `mapstructure:"dynamicports" default:"false"`
	BaseImage         string `mapstructure:"baseimage" default:"cloudfoundry/cflinuxfs3:latest"`
}

type Logging struct {
	Level  string `mapstructure:"level" valid:"in(debug|info|warn|error|panic|fatal),required" default:"info" flag:"program log level"`
	Output string `mapstructure:"output" valid:"required" default:"split"`
}

// Config the application's configuration
type Config struct {
	Log           Logging       `mapstructure:"log"`
	Team          string        `mapstructure:"team" valid:"required" flag:"team"`
	Deployment    Deployment    `mapstructure:"deployment"`
	KubeVela      *KubeVela     `mapstructure:"kubevela"`
	Docker        Docker        `mapstructure:"docker"`
	CF            CF            `mapstructure:"cf" valid:"required"`
	DockerStaging DockerStaging `mapstructure:"dockerstaging"`
}
