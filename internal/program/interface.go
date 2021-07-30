package program

type ProgramCLI interface {
	Init()
	LoadConfig() error
	GetJsonConfig() ([]byte, error)
	GenerateManifest() error
	PushApp() error
	BuildAppImage() error
	StageAppImage() error
	UploadAppImage() error
	RunAppImage(env map[string]string) error
}
