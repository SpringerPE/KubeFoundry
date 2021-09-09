package dockerstaging

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	config "kubefoundry/internal/config"
	log "kubefoundry/internal/log"
	cfmanifest "kubefoundry/internal/manifests"
	staging "kubefoundry/internal/staging"
	tar "kubefoundry/pkg/tar"

	dockertypes "github.com/docker/docker/api/types"
	dockertypescontainer "github.com/docker/docker/api/types/container"
	dockertypesnetwork "github.com/docker/docker/api/types/network"
	docker "github.com/docker/docker/client"
	dockererrors "github.com/docker/docker/errdefs"
	dockernat "github.com/docker/go-connections/nat"
	jsonmessage "github.com/moby/moby/pkg/jsonmessage"
	term "github.com/moby/term"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func init() {
	// Register this Driver
	staging.RegisterStagingDriver("DockerStaging", &DockerStaging{})
}

const (
	DockerContainerAppDir     = "/app"
	DockerConatinerBPDir      = "/buildpacks"
	DockerConatinerPersistDir = "/var/vcap/data"
	DockerContainerDockerFile = "Dockerfile"
	DockerContainerBaseImage  = "cloudfoundry/cflinuxfs3:latest"
)

type DockerStagingConfig struct {
	Registry                      string
	Username                      string
	Password                      string
	RemoveBeforeBuild             bool
	ContainerPersistentMountpoint string
	ContainerRestartPolicy        string // "unless-stopped", "no"
	ContainerDynamicPorts         bool
	ContainerBaseImage            string
	BPCacheDir                    string
}

type DockerStaging struct {
	cli                 *docker.Client
	config              *DockerStagingConfig
	appContainerDir     string
	bpContainerDir      string
	persistContainerDir string
	dockerfile          string
	log                 log.Logger
	contextData         *cfmanifest.ContextData
}

func (ds *DockerStaging) New(c *config.Config, l log.Logger) (staging.AppStaging, error) {
	dockerStgConfig := &DockerStagingConfig{
		Registry:                      c.Docker.Registry,
		Username:                      c.Docker.Username,
		Password:                      c.Docker.Password,
		RemoveBeforeBuild:             c.DockerStaging.RemoveBeforeBuild,
		ContainerPersistentMountpoint: DockerConatinerPersistDir,
		ContainerRestartPolicy:        c.DockerStaging.RestartPolicy,
		ContainerDynamicPorts:         c.DockerStaging.DynamicPorts,
		ContainerBaseImage:            c.DockerStaging.BaseImage,
	}
	cli, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
	if err != nil {
		err = fmt.Errorf("Unable to get connection with Docker: %s", err.Error())
		l.Error(err)
		return nil, err
	}
	l.Debugf("Connected with Docker server at '%s' running version %s", cli.DaemonHost(), cli.ClientVersion())
	dc := &DockerStaging{
		cli:                 cli,
		config:              dockerStgConfig,
		appContainerDir:     DockerContainerAppDir,
		bpContainerDir:      DockerConatinerBPDir,
		persistContainerDir: DockerConatinerPersistDir,
		dockerfile:          DockerContainerDockerFile,
		log:                 l,
	}
	return dc, nil
}

type DockerAppContainerImage struct {
	*DockerStaging
	baseImage string
	appData   *cfmanifest.AppData
	name      string
	output    io.Writer
	tags      []string
}

func (ds *DockerStaging) Stager(data *cfmanifest.ContextData, output io.Writer) (appPackages []staging.AppPackage, err error) {
	ds.contextData = data
	if data.CF == nil {
		panic("Not initialized context.Data")
	}
	if data.CF.Manifest == nil {
		panic("Not initialized context.Data.CF")
	}
	err = fmt.Errorf("")
	errors := false
	for _, app := range data.Apps {
		if _, errA := data.CF.Manifest.GetApplication(app.Name); errA == nil {
			appPackage := ds.NewDockerAppContainerImage(app, output)
			appPackages = append(appPackages, appPackage)
		} else {
			errors = true
			err = fmt.Errorf("%s\n %s", err.Error(), errA.Error())
		}
	}
	if errors {
		return appPackages, err
	}
	return appPackages, nil
}

func (ds *DockerStaging) Finish(ctx context.Context, appPackages []staging.AppPackage) (err error) {
	err = fmt.Errorf("Unable to clean resources: ")
	errors := false
	for _, app := range appPackages {
		errA := app.Destroy(ctx, true)
		if errA != nil {
			errors = true
			err = fmt.Errorf("%s\n %s", err.Error(), errA.Error())
		}
	}
	ds.cli.Close()
	if errors {
		return err
	}
	return nil
}

func (ds *DockerStaging) NewDockerAppContainerImage(appData *cfmanifest.AppData, output io.Writer) (appPackage *DockerAppContainerImage) {
	appPackage = &DockerAppContainerImage{
		DockerStaging: ds,
		appData:       appData,
		output:        output,
		name:          appData.Name,
		tags:          []string{appData.Name},
	}
	return
}

// Pull images
func (ac *DockerAppContainerImage) Pull(ctx context.Context, baseImage bool) (err error) {
	image := ac.appData.Image
	if baseImage {
		image = ac.config.ContainerBaseImage
	}
	ac.log.Infof("Pulling Docker image '%s' ...", image)
	registryAuth := ""
	imageRegistry := strings.SplitN(image, "/", 2)
	if strings.Contains(ac.config.Registry, imageRegistry[0]) {
		authConfig := dockertypes.AuthConfig{
			Username:      ac.config.Username,
			Password:      ac.config.Password,
			ServerAddress: ac.config.Registry,
		}
		authConfigBytes, _ := json.Marshal(authConfig)
		registryAuth = base64.URLEncoding.EncodeToString(authConfigBytes)
	}
	pullOpts := dockertypes.ImagePullOptions{
		RegistryAuth: registryAuth,
		All:          false,
	}
	if pullResponse, err := ac.cli.ImagePull(ctx, image, pullOpts); err != nil {
		err = fmt.Errorf("Unable to pull image '%s': %s", image, err.Error())
		ac.log.Error(err)
	} else {
		defer pullResponse.Close()
		err = ac.displayJSONMessagesStream(pullResponse, ac.output)
		if err == nil {
			// Tag the image
			err = ac.cli.ImageTag(ctx, image, ac.name)
			if err != nil {
				err = fmt.Errorf("Unable to tag image '%s' with '%s': %s", image, ac.name, err.Error())
				ac.log.Error(err)
			}
		}
	}
	return
}

func (ac *DockerAppContainerImage) Build(ctx context.Context) (id string, err error) {
	id = ""
	appbits, errB := os.Stat(ac.appData.Dir)
	if os.IsNotExist(errB) {
		err = fmt.Errorf("Applicaction path '%s' does not exist", ac.appData.Dir)
		ac.log.Error(err)
		return
	}
	// Pull base image
	err = ac.Pull(ctx, true)
	if err != nil {
		return
	}
	// Create a tar with the app for docker context folder
	app_bits := ac.appContainerDir
	if appbits.IsDir() {
		ac.log.Infof("Packaging application context dir '%s' ...", ac.appData.Dir)
		app_bits = "."
	} else {
		ac.log.Infof("Packaging application context file '%s' ...", ac.appData.Dir)
		app_bits = filepath.Base(ac.appData.Dir)
	}
	tarcontext := bytes.NewBuffer(nil)
	tar := tar.NewTar(".", ac.log, tarcontext)
	tar.Add(ctx, ac.appData.Dir, ac.appContainerDir)
	if !appbits.IsDir() {
		// Add the manifest
		appManifest := filepath.Join(ac.contextData.CF.Manifest.Path, ac.contextData.CF.Manifest.Filename)
		tar.Add(ctx, appManifest, ac.appContainerDir)
	}
	// Caching mechanism and way to provide buildpacks directly to the container
	if ac.config.BPCacheDir != "" {
		if _, err = os.Stat(ac.config.BPCacheDir); !os.IsNotExist(err) {
			tar.Add(ctx, ac.config.BPCacheDir, ac.bpContainerDir)
		} else if err != nil {
			tar.Close()
			return
		}
	}
	err = IterateEmbedStaging(tar.AddFile)
	tar.Close()
	if err != nil {
		return
	}
	// Docker build options
	buildArgs := make(map[string]*string)
	for key, value := range ac.contextData.Args {
		buildArgs[key] = &value
	}
	ac.log.Infof("Building Docker container image '%s' (%s:%s) ...", ac.name, ac.appData.Name, ac.appData.Version)
	buildArgs["BASE"] = &ac.config.ContainerBaseImage
	buildArgs["CONTEXT_DIR"] = &ac.appContainerDir
	buildArgs["BUILDPACKS_DIR"] = &ac.bpContainerDir
	buildArgs["APP_BITS"] = &app_bits
	buildArgs["APP_NAME"] = &ac.appData.Name
	buildArgs["APP_CREATED"] = &ac.contextData.DateHuman
	buildArgs["APP_VERSION"] = &ac.appData.Version
	app_port := strconv.Itoa(ac.appData.Port)
	buildArgs["APP_PORT"] = &app_port
	buildArgs["CF_MANIFEST"] = &ac.contextData.CF.Manifest.Filename
	buildArgs["CF_API"] = &ac.contextData.CF.Api
	buildArgs["CF_ORG"] = &ac.contextData.CF.Org
	buildArgs["CF_SPACE"] = &ac.contextData.CF.Space
	imageBuildOptions := dockertypes.ImageBuildOptions{
		Remove:         ac.config.RemoveBeforeBuild,
		ForceRemove:    true,
		PullParent:     true,
		SuppressOutput: false,
		Dockerfile:     ac.dockerfile,
		Tags:           ac.tags,
		BuildArgs:      buildArgs,
		Squash:         false,
	}
	if buildResponse, err := ac.cli.ImageBuild(ctx, tarcontext, imageBuildOptions); err != nil {
		err = fmt.Errorf("Unable to run CF staging for '%s': %s", ac.name, err.Error())
		ac.log.Error(err)
	} else {
		defer buildResponse.Body.Close()
		if err = ac.displayJSONMessagesStream(buildResponse.Body, ac.output); err != nil {
			err = fmt.Errorf("Doker CF staging error: %s", err.Error())
			ac.log.Error(err)
			return id, err
		}
		// Get image details - this will check if image build was successful
		image, _, err := ac.cli.ImageInspectWithRaw(ctx, ac.name)
		if err != nil {
			err = fmt.Errorf("Staging process build not completed: %s", err.Error())
			ac.log.Error(err)
			return id, err
		}
		id = image.ID
	}
	return
}

func (ac *DockerAppContainerImage) Destroy(ctx context.Context, all bool) (err error) {
	ac.log.Info("Stopping and cleaning resources for '%s' (%s) ...", ac.name, strconv.FormatBool(all))
	rmOptions := dockertypes.ContainerRemoveOptions{
		RemoveVolumes: all,
		Force:         true,
	}
	err = ac.cli.ContainerRemove(ctx, ac.name, rmOptions)
	if err != nil {
		err = fmt.Errorf("Unable to remove (running?) container: %s", err.Error())
		ac.log.Error(err)
	}
	// Get image details - this will check if image build was successful
	if image, _, erri := ac.cli.ImageInspectWithRaw(ctx, ac.name); erri != nil {
		err = fmt.Errorf("Unknown image '%s': %s", ac.name, erri.Error())
		ac.log.Error(err)
	} else {
		delOptions := dockertypes.ImageRemoveOptions{
			PruneChildren: all,
			Force:         true,
		}
		_, err = ac.cli.ImageRemove(ctx, image.ID, delOptions)
		if err != nil {
			err = fmt.Errorf("Unable to remove image: %s", err.Error())
			ac.log.Error(err)
		}
	}
	return
}

func (ac *DockerAppContainerImage) Info(ctx context.Context) (info map[string]interface{}, err error) {
	info = make(map[string]interface{})
	// Get image details - this will check if image build was successful
	if image, _, erri := ac.cli.ImageInspectWithRaw(ctx, ac.name); erri != nil {
		err = fmt.Errorf("Unknown image '%s': %s", ac.name, erri.Error())
		ac.log.Error(err)
	} else {
		info["name"] = ac.name
		info["id"] = image.ID
		info["tags"] = image.RepoTags
		info["parent"] = image.Parent
		info["comment"] = image.Comment
		info["created"] = image.Created
		info["author"] = image.Author
		info["size"] = image.Size
		info["architecture"] = image.Architecture
		info["os"] = image.Os
	}
	return
}

func (ac *DockerAppContainerImage) Push(ctx context.Context) (err error) {
	// Tag the image
	ac.log.Infof("Pushing image '%s' to '%s' ...", ac.name, ac.appData.Image)
	err = ac.cli.ImageTag(ctx, ac.name, ac.appData.Image)
	if err != nil {
		err = fmt.Errorf("Unable to tag image '%s' with '%s': %s", ac.name, ac.appData.Image, err.Error())
		ac.log.Error(err)
		return
	}
	// Push to the registry
	registryAuth := ""
	imageRegistry := strings.SplitN(ac.appData.Image, "/", 2)
	if strings.Contains(ac.config.Registry, imageRegistry[0]) {
		authConfig := dockertypes.AuthConfig{
			Username:      ac.config.Username,
			Password:      ac.config.Password,
			ServerAddress: ac.config.Registry,
		}
		authConfigBytes, _ := json.Marshal(authConfig)
		registryAuth = base64.URLEncoding.EncodeToString(authConfigBytes)
	}
	pushOpts := dockertypes.ImagePushOptions{
		RegistryAuth: registryAuth,
	}
	if pushResponse, errp := ac.cli.ImagePush(ctx, ac.appData.Image, pushOpts); errp != nil {
		err = fmt.Errorf("Unable to push image '%s' to '%s': %s", ac.appData.Image, ac.config.Registry, errp.Error())
		ac.log.Error(err)
	} else {
		defer pushResponse.Close()
		if errd := ac.displayJSONMessagesStream(pushResponse, ac.output); errd != nil {
			err = fmt.Errorf("Push error message: %s", errd.Error())
			ac.log.Error(err)
		} else {
			ac.tags = append(ac.tags, ac.appData.Image)
		}
	}
	return
}

func (ac *DockerAppContainerImage) Run(ctx context.Context, dataDir string, env map[string]string, output bool) (err error) {
	image, _, erri := ac.cli.ImageInspectWithRaw(ctx, ac.name)
	if erri != nil {
		err = fmt.Errorf("Unknown image '%s': %s", ac.name, erri.Error())
		ac.log.Error(err)
		return
	}
	containerhost := ac.name
	ac.log.Infof("Running image '%s' tailing output, in container '%s' ...", ac.name, containerhost)
	portMap := dockernat.PortMap{}
	for p := range image.Config.ExposedPorts {
		newport, err := dockernat.NewPort("tcp", p.Port())
		if err != nil {
			err = fmt.Errorf("Unable to setup docker networking for container '%s' : %s", containerhost, err.Error())
			ac.log.Error(err)
			return err
		}
		//portDef := dockernat.PortBinding{HostIP: "0.0.0.0"}
		portDef := dockernat.PortBinding{}
		if !ac.config.ContainerDynamicPorts {
			portDef.HostPort = p.Port()
		}
		portMap[newport] = []dockernat.PortBinding{portDef}
	}
	volumeBindings := []string{}
	if ac.persistContainerDir != "" && dataDir != "" {
		volumeBindings = append(volumeBindings, dataDir+":"+ac.persistContainerDir)
	}
	resources := dockertypescontainer.Resources{}
	cpuResources, errC := strconv.ParseFloat(ac.appData.Resources.CPU, 64)
	if errC != nil {
		ac.log.Warnf("Unable to apply cpu limits: %s", errC.Error())
	}
	memoryResources, errM := strconv.ParseInt(ac.appData.Resources.Mem, 10, 64)
	if errM != nil {
		ac.log.Warnf("Unable to apply memory limits: %s", errM.Error())
	}
	if errM == nil && errC == nil {
		resources = dockertypescontainer.Resources{
			Memory:   memoryResources,
			NanoCPUs: int64(cpuResources * (math.Pow(10, 9))),
		}
	}
	hostConfig := dockertypescontainer.HostConfig{
		AutoRemove: false,
		Binds:      volumeBindings,
		// PublishAllPorts:  true,
		PortBindings: portMap,
		LogConfig:    dockertypescontainer.LogConfig{},
		Resources:    resources,
		RestartPolicy: dockertypescontainer.RestartPolicy{
			Name: ac.config.ContainerRestartPolicy,
		},
	}
	networkConfig := dockertypesnetwork.NetworkingConfig{}
	specs := ocispec.Platform{
		Architecture: image.Architecture,
		OS:           image.Os,
	}
	kubefoundryEnv := map[string]string{
		"APP_NAME":    ac.appData.Name,
		"APP_CREATED": ac.contextData.DateHuman,
		"APP_VERSION": ac.appData.Version,
		"APP_PORT":    strconv.Itoa(ac.appData.Port),
		"CF_MANIFEST": ac.contextData.CF.Manifest.Filename,
		"CF_API":      ac.contextData.CF.Api,
		"CF_ORG":      ac.contextData.CF.Org,
		"CF_SPACE":    ac.contextData.CF.Space,
	}
	envlist := []string{}
	for k, v := range env {
		envlist = append(envlist, fmt.Sprintf("%s=%s", k, v))
	}
	for k, v := range kubefoundryEnv {
		envlist = append(envlist, fmt.Sprintf("%s=%s", k, v))
	}
	_, isTerminal := term.GetFdInfo(ac.output)
	config := dockertypescontainer.Config{
		Hostname:     containerhost,
		AttachStdout: output,
		AttachStderr: output,
		Tty:          isTerminal,
		Image:        image.ID,
		Env:          envlist,
	}
	containerResp, err := ac.cli.ContainerCreate(ctx, &config, &hostConfig, &networkConfig, &specs, containerhost)
	if err != nil {
		if dockererrors.IsConflict(err) {
			rmOptions := dockertypes.ContainerRemoveOptions{
				RemoveVolumes: false,
				Force:         true,
			}
			err = ac.cli.ContainerRemove(ctx, containerhost, rmOptions)
			if err != nil {
				err = fmt.Errorf("Unable to remove (running?) container: %s", err.Error())
				ac.log.Error(err)
				return
			}
			containerResp, err = ac.cli.ContainerCreate(ctx, &config, &hostConfig, &networkConfig, &specs, containerhost)
		}
		if err != nil {
			err = fmt.Errorf("Unable to create and run container '%s': %s", containerhost, err.Error())
			ac.log.Error(err)
			return
		}
	}
	if len(containerResp.Warnings) > 0 {
		ac.log.Warn(strings.Join(containerResp.Warnings, "\n"))
	}
	err = ac.cli.ContainerStart(ctx, containerResp.ID, dockertypes.ContainerStartOptions{})
	if err != nil {
		err = fmt.Errorf("Unable to start container '%s': %s", containerhost, err.Error())
		ac.log.Error(err)
		return
	}
	if output {
		options := dockertypes.ContainerLogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Timestamps: true,
			Follow:     true,
		}
		out, errLogs := ac.cli.ContainerLogs(ctx, containerResp.ID, options)
		if errLogs != nil {
			err = fmt.Errorf("Unable to get stdout/stderr from container '%s': %s", containerhost, errLogs.Error())
			ac.log.Error(err)
		} else {
			go func() {
				if _, err := io.Copy(ac.output, out); err != nil {
					ac.log.Error(err)
				}
			}()
		}
	}
	// Get the port on docker server
	if info, err := ac.cli.ContainerInspect(ctx, containerResp.ID); err == nil {
		ac.log.Debugf("Docker GW ip: %s", info.NetworkSettings.Gateway)
		for port, host := range info.NetworkSettings.Ports {
			ac.log.Infof("Application running on container '%s', internal port %s availabe at http://%s:%s", containerhost, port, host[0].HostIP, host[0].HostPort)
		}
	}
	statusCh, errCh := ac.cli.ContainerWait(ctx, containerResp.ID, dockertypescontainer.WaitConditionNotRunning)
	select {
	case err = <-errCh:
		// check if context cancelled, user pressed ctr-c
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				err = fmt.Errorf("Application in container '%s' does not start: %s", containerhost, err.Error())
				ac.log.Error(err)
			}
		}
		err = ac.cli.ContainerStop(context.Background(), containerResp.ID, nil)
		if err != nil {
			err = fmt.Errorf("Unable to stop application container: %s", err.Error())
			ac.log.Error(err)
		}
	case status := <-statusCh:
		ac.log.Infof("Application in container '%s' exited with status %s", containerhost, status.StatusCode)
	}
	return err
}

func (ac *DockerAppContainerImage) print(out io.Writer, info bool, msg, color string) {
	fdinfo, isTerminal := term.GetFdInfo(out)
	if out != nil && isTerminal {
		size, err := term.GetWinsize(fdinfo)
		terLong := uint16(80)
		if err == nil {
			terLong = size.Width
		}
		if info {
			// Skip all docker messages about intermediate containers
			if !strings.HasPrefix(msg, "---> ") && !strings.HasPrefix(msg, "Removing intermediate container") {
				msgLong := uint16(len(msg))
				if msgLong > terLong {
					msgLong = terLong
				}
				verb := fmt.Sprintf("%%%ds", -int(terLong))
				if color != "" {
					fmt.Fprintf(out, fmt.Sprintf(verb, color+msg[:msgLong])+"\033[0m\r")
				} else {
					fmt.Fprintf(out, fmt.Sprintf(verb, msg[:msgLong])+"\r")
				}
			}
		} else {
			verb := fmt.Sprintf("%%%ds", int(terLong)+1)
			if color != "" {
				fmt.Fprintf(out, fmt.Sprintf(verb, "\r")+color+msg+"\033[0m")
			} else {
				fmt.Fprintf(out, fmt.Sprintf(verb, "\r")+msg)
			}
		}
	}
}

// DisplayJSONMessagesStream displays a json message stream from `in` to `out`, `isTerminal`
// describes if `out` is a terminal. If this is the case, it will print `\n` at the end of
// each line and move the cursor while displaying.
func (ac *DockerAppContainerImage) displayJSONMessagesStream(in io.Reader, out io.Writer) error {
	_, isTerminal := term.GetFdInfo(out)
	dec := json.NewDecoder(in)
	stgrunning := false
	status := ""
	progress := false
	for {
		var jm jsonmessage.JSONMessage
		if err := dec.Decode(&jm); err != nil {
			if err == io.EOF {
				break
			}
			err = fmt.Errorf("Error decoding Docker API message: %s", err.Error())
			ac.log.Error(err)
			return err
		}
		if jm.Error != nil {
			if jm.Error.Code == 401 {
				err := fmt.Errorf("Docker API authentication error: %s", jm.ErrorMessage)
				ac.log.Error(err)
				return err
			}
			ac.log.Error(jm.Error)
			return jm.Error
		} else {
			stream := strings.TrimSpace(jm.Stream)
			if stream != "" {
				if !stgrunning {
					if strings.HasPrefix(stream, "#--- MSG!") {
						ac.print(out, true, "", "")
						ac.log.Info(stream[10:])
					} else if strings.HasPrefix(stream, "#--- SRT!") {
						ac.print(out, true, "", "")
						ac.log.Info(stream[10:])
						stgrunning = true
					} else {
						if strings.HasPrefix(stream, "Successfully") {
							ac.print(out, true, "", "")
							ac.log.Info(stream)
						} else {
							ac.log.Debug(stream)
							ac.print(out, true, stream, "\033[1;36m")
						}
					}
				} else {
					if strings.HasPrefix(stream, "#--- END!") {
						ac.print(out, true, "", "")
						ac.log.Info(stream[10:])
						stgrunning = false
					} else {
						//ac.log.Debug(stream)
						//ac.print(out, false, stream+"\n", "\033[1;36m")
						ac.log.Info("\033[1;36m" + stream + "\033[0m")
					}
				}
			}
			if jm.Status != "" {
				if isTerminal {
					if jm.Status != status {
						ac.log.Debug(jm.Status)
						//print(false, status)
						if progress {
							progress = false
							ac.print(out, false, "\r", "\033[1;36m")
						}
					}
					if jm.ProgressMessage != "" {
						ac.print(out, false, jm.Status+" "+jm.ProgressMessage+"\r", "\033[1;36m")
						progress = true
					}
				} else if jm.Status != status {
					ac.log.Debug(jm.Status)
					//ac.print(out, false, jm.Status+"\n", "")
				}
				status = jm.Status
			}
			if jm.Aux != nil {
				var result dockertypes.BuildResult
				if err := json.Unmarshal(*jm.Aux, &result); err != nil {
					err = fmt.Errorf("Failed to parse AUX message: %s", err.Error())
					ac.log.Error(err)
					return err
				}
				ac.log.Debug("Image checksum " + string(result.ID))
			}
		}
	}
	return nil
}
