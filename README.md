KubeFoundry
===========

Kubernetes/KubeVela + CloudFoundry staging. It stages applications in the same way as CloudFoundry (traditional buildpacks, non Cloud Native) generating a runable Docker Image (also pushing it to the remote registry)
It also generates manifests for Kubevela and Kubernetes (StatefulSet) and it pushes those definitions to the PaaS. 

Usage
=====

```
Deploy CloudFoundry applications to Kubevela with style

Usage:
   [command]

Available Commands:
  build       Build Kubevela application container image
  config      Shows digested configuration
  help        Help about any command
  manifest    Generate Kubevela manifest(s)
  push        Push application to the PaaS
  run         Run application locally using docker
  stage       Build and Push Kubevela application container image
  version     Show build and version

Flags:
      --config string                          yaml config file (default /home/jriguera/.kubefoundry/config.yml)
      --deployment.appname string              app
      --deployment.appversion string           version
      --deployment.args string                 args
      --deployment.defaults.cpu string         default cpu
      --deployment.defaults.disk string        default disk
      --deployment.defaults.domain string      default domain
      --deployment.defaults.mem string         default mem
      --deployment.defaults.port string        default port
      --deployment.manifest.appfile string     kubevela appfile
      --deployment.manifest.generate string    manifest generate
      --deployment.manifest.overwrite string   manifest overwrite
      --deployment.manifest.parsecf string     cf manifest
      --deployment.path string                 path
      --deployment.registry string             registry prefix
      --deployment.stagingdriver string        staging
      --docker.api string                      docker api
  -h, --help                                   help for this command
      --kubevela.environment string            Kubevela environment
      --kubevela.kubeconfig string             Kubernetes config
      --kubevela.namespace string              Kubevela namespace
      --log.level string                       program log level
      --team string                            team

Use " [command] --help" for more information about a command.
```


Configuration
-------------

See example `config.yml`  and for all options `internal/config/config.go`
You can put the file `config.yml` in the same folder where the binary is executed or place it in `~/.kubefoundry/config.yml`. You may need to provide the Docker Registry settings in order to be able to push the images to it. Also, you may
need to update the `Deployment` settings: `Registry` and `Domain`.

For now, the only staging implementation is `DockerStaging`.

You can check the configuration by running `kubefoundry config`


Usage
-----

Once you have the correct configuration file. Go to the repository where you execute `cf push` and run:

1. Run `kubefoundry build`  to generate the Docker image with the application ready to run. If the process is successful you should see the image with `docker images`.
2. Run `kubefoundry run` to get the application running with the same memory settings as in the manifest (and 1 cpu per 1G).
You can also run the image with `docker run -ti --rm -p 8080:8080 <app-name>`.
3. Run `kubefoundry manifest` to generate Kubevela and K8S manifests to pass to `vela` (`vela up`) or `kubectl` (`kubectl apply -f deploy.yml`).

You can also use `kubefoundry stage` to build and push the image to the remote registry.

The push functionality is not ready yet.

Example:
```
$ kubefoundry --cf.manifest manifest-test.yml  --deployment.apppath searchdirect-ci.zip stage
9098-09-09/13:19:27 Configuration loaded from file: /home/jriguera/.kubefoundry/config.yml
9098-09-09/13:19:27 Pulling Docker image 'cloudfoundry/cflinuxfs3:latest' ...
9098-09-09/13:19:29 Packaging application context file 'searchdirect-ci.zip' ...
9098-09-09/13:19:29 Building Docker container image 'searchdirect-qa' (searchdirect-qa:82214d95e072) ...
9098-09-09/13:19:39 Staging application in Docker container ...                                                                                                                                                                               
9098-09-09/13:19:39 Application 'searchdirect-qa' buildpacks: ['https://github.com/cloudfoundry/java-buildpack.git#v4.41']
9098-09-09/13:19:39 Downloading buildpack 'https://github.com/cloudfoundry/java-buildpack.git#v4.41' (https://github.com/cloudfoundry/java-buildpack.git) ...
9098-09-09/13:19:41 Running staging process with buildpack #0: https://github.com/cloudfoundry/java-buildpack.git#v4.41
9098-09-09/13:19:42 [STG.fin]  -----> Java Buildpack v4.41
9098-09-09/13:19:43 [STG.fin]  -----> Downloading Jvmkill Agent 1.16.0_RELEASE from https://java-buildpack.cloudfoundry.org/jvmkill/bionic/x86_64/jvmkill-1.16.0-RELEASE.so (0.3s)
9098-09-09/13:19:46 [STG.fin]  -----> Downloading Open Jdk JRE 16.0.2_7 from https://java-buildpack.cloudfoundry.org/openjdk/bionic/x86_64/bellsoft-jre16.0.2%2B7-linux-amd64.tar.gz (3.2s)
9098-09-09/13:19:47 [STG.fin]         Expanding Open Jdk JRE to .java-buildpack/open_jdk_jre (1.1s)
9098-09-09/13:19:47 [STG.fin]  -----> Downloading Open JDK Like Memory Calculator 3.13.0_RELEASE from https://java-buildpack.cloudfoundry.org/memory-calculator/bionic/x86_64/memory-calculator-3.13.0-RELEASE.tar.gz (0.1s)
9098-09-09/13:19:48 [STG.fin]         Loaded Classes: 21102, Threads: 250
9098-09-09/13:19:48 [STG.fin]  -----> Downloading Client Certificate Mapper 1.11.0_RELEASE from https://java-buildpack.cloudfoundry.org/client-certificate-mapper/client-certificate-mapper-1.11.0-RELEASE.jar (0.0s)
9098-09-09/13:19:48 [STG.fin]  -----> Downloading Container Security Provider 1.18.0_RELEASE from https://java-buildpack.cloudfoundry.org/container-security-provider/container-security-provider-1.18.0-RELEASE.jar (0.1s)
9098-09-09/13:19:50 Application 'searchdirect-qa' successfully staged/compiled
9098-09-09/13:19:50 Application 'searchdirect-qa' startup command: JAVA_OPTS="-agentpath:$PWD/.java-buildpack/open_jdk_jre/bin/jvmkill-1.16.0_RELEASE=printHeapHistogram=1 -Djava.io.tmpdir=$TMPDIR -XX:ActiveProcessorCount=$(nproc) -Djava.ext.dirs= -Djava.security.properties=$PWD/.java-buildpack/java_security/java.security $JAVA_OPTS" && CALCULATED_MEMORY=$($PWD/.java-buildpack/open_jdk_jre/bin/java-buildpack-memory-calculator-3.13.0_RELEASE -totMemory=$MEMORY_LIMIT -loadedClasses=21231 -poolType=metaspace -stackThreads=250 -vmOptions="$JAVA_OPTS") && echo JVM Memory Configuration: $CALCULATED_MEMORY && JAVA_OPTS="$JAVA_OPTS $CALCULATED_MEMORY" && MALLOC_ARENA_MAX=2 JAVA_OPTS=$JAVA_OPTS JAVA_HOME=$PWD/.java-buildpack/open_jdk_jre exec $PWD/bin/searchdirect
9098-09-09/13:19:51 Finished Cloudfoundry staging process.                                                                                                                                                                                    
9098-09-09/13:19:54 Creating final Docker container image ...                                                                                                                                                                                 
9098-09-09/13:20:12 Created Docker container image for application                                                                                                                                                                            
9098-09-09/13:20:14 Successfully built 676c1d333268                                                                                                                                                                                           
9098-09-09/13:20:14 Successfully tagged searchdirect-qa:latest                                                                                                                                                                                
9098-09-09/13:20:14 Pushing image 'searchdirect-qa' to 'eu.gcr.io/halfpipe-io/engineering-enablement/searchdirect-qa:82214d95e072' ...


$ kubefoundry run
3098-09-03/14:48:09 Configuration loaded from file: /home/jriguera/.kubefoundry/config.yml
3098-09-03/14:48:09 Running image 'test-app' tailing output, in container 'test-app' ...
3098-09-03/14:48:09 Your kernel does not support swap limit capabilities or the cgroup is not mounted. Memory limited without swap.
3098-09-03/14:48:10 Application running on container 'test-app', internal port 8080/tcp availabe at http://0.0.0.0:49153
2021-09-03T12:48:10.224237324Z Loading /home/vcap/app/.profile.d/000_multi-supply.sh
2021-09-03T12:48:10.225623672Z Loading /home/vcap/app/.profile.d/0_python.fixeggs.sh
2021-09-03T12:48:10.244251494Z Loading /home/vcap/app/.profile.d/0_python.sh
2021-09-03T12:48:10.528340652Z  * Serving Flask app 'app' (lazy loading)
2021-09-03T12:48:10.528364696Z  * Environment: production
2021-09-03T12:48:10.528369181Z    WARNING: This is a development server. Do not use it in a production deployment.
2021-09-03T12:48:10.528374339Z    Use a production WSGI server instead.
2021-09-03T12:48:10.528377252Z  * Debug mode: off
2021-09-03T12:48:10.530638918Z  * Running on all addresses.
2021-09-03T12:48:10.530659106Z    WARNING: This is a development server. Do not use it in a production deployment.
2021-09-03T12:48:10.530738543Z  * Running on http://172.17.0.2:8080/ (Press CTRL+C to quit)
2021-09-03T12:48:25.056898258Z 172.17.0.1 - - [03/Sep/2021 12:48:25] "GET / HTTP/1.1" 200 -
2021-09-03T12:48:25.308732846Z 172.17.0.1 - - [03/Sep/2021 12:48:25] "GET /favicon.ico HTTP/1.1" 404 -
^C3098-09-03/14:48:34 context canceled

$ docker images
REPOSITORY                                                            TAG                    IMAGE ID       CREATED         SIZE
test-app                                                              latest                 0c1529c67b3e   5 minutes ago   1.24GB
<none>                                                                <none>                 db3aa34f24cd   5 minutes ago   1.61GB
mongo                                                                 latest                 0bcbeb494bed   3 days ago      684MB
cloudfoundry/cflinuxfs3                                               latest                 75b472ce7452   7 days ago      1.06GB

$ docker run -ti --rm -p 8080:8080 test-app
Loading /home/vcap/app/.profile.d/000_multi-supply.sh
Loading /home/vcap/app/.profile.d/0_python.fixeggs.sh
Loading /home/vcap/app/.profile.d/0_python.sh
 * Serving Flask app 'app' (lazy loading)
 * Environment: production
   WARNING: This is a development server. Do not use it in a production deployment.
   Use a production WSGI server instead.
 * Debug mode: off
 * Running on all addresses.
   WARNING: This is a development server. Do not use it in a production deployment.
 * Running on http://172.17.0.2:8080/ (Press CTRL+C to quit)
^CPropagating signal '2' to all children ...
Application 0_test-app (pid=10) exited with code 0

```

You can also generate manifests and deploy to Kubernetes using `kubectl` directly. The manifest needed to deploy directly to kubernetes is `deploy.yml`:

```
$ kubefoundry --cf.manifest manifest-test.yml  --deployment.apppath searchdirect-ci.zip manifest
9098-09-09/13:21:55 Configuration loaded from file: /home/jriguera/.kubefoundry/config.yml
9098-09-09/13:21:55 Generating AppFile manifest: /home/jriguera/devel/work/oscar/searchdirect/jose-searchdirect/vela.yml
9098-09-09/13:21:55 Generating KubeFoundry manifest: /home/jriguera/devel/work/oscar/searchdirect/jose-searchdirect/app.yml
9098-09-09/13:21:55 Generating K8S manifest: /home/jriguera/devel/work/oscar/searchdirect/jose-searchdirect/deploy.yml
$ kubectl apply --validate=true -f deploy.yml 
service/jose-searchdirect created
virtualservice.networking.istio.io/jose-searchdirect created
statefulset.apps/jose-searchdirect created
```




Development
===========

Golang >= 1.16 . There is a `Makefile` to manage the development actions, releases
and binaries. `make build` generates binaries for Linux, Darwin and Windows architectures.

Golang Modules
--------------

```
go mod init         # If you are not using git, type `go mod init $(basename `pwd`)`
go mod vendor       # if you have vendor/ folder, will automatically integrate
go build
```

This method creates a file called `go.mod` in your projects directory. You can
then build your project with `go build`.


Author
======

(c) 2021 Springer Nature, Engineering Enablement, Jose Riguera

Apache 2.0
