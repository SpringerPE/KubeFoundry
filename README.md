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
$ kubefoundry  build
3098-09-03/14:44:44 Configuration loaded from file: /home/jriguera/.kubefoundry/config.yml
3098-09-03/14:44:44 Pulling Docker image 'cloudfoundry/cflinuxfs3:latest' ...
Downloading [=================================================> ]  362.6MB/363.8MB
Extracting [==================================================>]  363.8MB/363.8MB
3098-09-03/14:45:21 Packaging application context dir '/home/jriguera/devel/work/pe-bosh-workspace/test-application/app' ...
3098-09-03/14:45:21 Building docker image 'test-app' ...
3098-09-03/14:45:21 Running CloudFoundry staging to generate a container image ...
Application 'test-app' buildpacks: ['https://github.com/cloudfoundry/python-buildpack.git#v1.6.27']
Downloading buildpack 'https://github.com/cloudfoundry/python-buildpack.git#v1.6.27' ...
Running staging process with buildpack #0: https://github.com/cloudfoundry/python-buildpack.git#v1.6.27
[STG.sup]  -----> Download go 1.11.4
[STG.sup]  -----> Running go build supply
[STG.sup]  /buildpacks/test-app/0 /buildpacks/test-app/0
[STG.sup]  /buildpacks/test-app/0
[STG.sup]  -----> Python Buildpack version 1.6.27
[STG.sup]  -----> Supplying Python
[STG.sup]  -----> Installing python 3.7.2
[STG.sup]         Download [https://buildpacks.cloudfoundry.org/dependencies/python/python-3.7.2-linux-x64-cflinuxfs3-5bac6de6.tgz]
[STG.sup]  -----> Installing setuptools 40.6.3
[STG.sup]         Download [https://buildpacks.cloudfoundry.org/dependencies/setuptools/setuptools-40.6.3-any-stack-3b474dad.zip]
[STG.sup]  -----> Installing pip 9.0.3
[STG.sup]         Download [https://buildpacks.cloudfoundry.org/dependencies/pip/pip-9.0.3-7bf48f9a.tar.gz]
[STG.sup]  -----> Installing pip-pop 0.1.1
[STG.sup]         Download [https://buildpacks.cloudfoundry.org/dependencies/manual-binaries/pip-pop/pip-pop-0.1.1-d410583a.tar.gz]
[STG.sup]  -----> Running Pip Install
[STG.sup]         Collecting Flask>=0.12 (from -r /home/vcap/app/requirements.txt (line 1))
[STG.sup]           Downloading https://files.pythonhosted.org/packages/54/4f/1b294c1a4ab7b2ad5ca5fc4a9a65a22ef1ac48be126289d97668852d4ab3/Flask-2.0.1-py3-none-any.whl (94kB)
[STG.sup]         Collecting click>=7.1.2 (from Flask>=0.12->-r /home/vcap/app/requirements.txt (line 1))
[STG.sup]           Downloading https://files.pythonhosted.org/packages/76/0a/b6c5f311e32aeb3b406e03c079ade51e905ea630fc19d1262a46249c1c86/click-8.0.1-py3-none-any.whl (97kB)
[STG.sup]         Collecting Jinja2>=3.0 (from Flask>=0.12->-r /home/vcap/app/requirements.txt (line 1))
[STG.sup]           Downloading https://files.pythonhosted.org/packages/80/21/ae597efc7ed8caaa43fb35062288baaf99a7d43ff0cf66452ddf47604ee6/Jinja2-3.0.1-py3-none-any.whl (133kB)
[STG.sup]         Collecting Werkzeug>=2.0 (from Flask>=0.12->-r /home/vcap/app/requirements.txt (line 1))
[STG.sup]           Downloading https://files.pythonhosted.org/packages/bd/24/11c3ea5a7e866bf2d97f0501d0b4b1c9bbeade102bb4b588f0d2919a5212/Werkzeug-2.0.1-py3-none-any.whl (288kB)
[STG.sup]         Collecting itsdangerous>=2.0 (from Flask>=0.12->-r /home/vcap/app/requirements.txt (line 1))
[STG.sup]           Downloading https://files.pythonhosted.org/packages/9c/96/26f935afba9cd6140216da5add223a0c465b99d0f112b68a4ca426441019/itsdangerous-2.0.1-py3-none-any.whl
[STG.sup]         Collecting importlib-metadata; python_version < "3.8" (from click>=7.1.2->Flask>=0.12->-r /home/vcap/app/requirements.txt (line 1))
[STG.sup]           Downloading https://files.pythonhosted.org/packages/71/c2/cb1855f0b2a0ae9ccc9b69f150a7aebd4a8d815bd951e74621c4154c52a8/importlib_metadata-4.8.1-py3-none-any.whl
[STG.sup]         Collecting MarkupSafe>=2.0 (from Jinja2>=3.0->Flask>=0.12->-r /home/vcap/app/requirements.txt (line 1))
[STG.sup]           Downloading https://files.pythonhosted.org/packages/d7/56/9d9c0dc2b0f5dc342ff9c7df31c523cc122947970b5ea943b2311be0c391/MarkupSafe-2.0.1-cp37-cp37m-manylinux1_x86_64.whl
[STG.sup]         Collecting zipp>=0.5 (from importlib-metadata; python_version < "3.8"->click>=7.1.2->Flask>=0.12->-r /home/vcap/app/requirements.txt (line 1))
[STG.sup]           Downloading https://files.pythonhosted.org/packages/92/d9/89f433969fb8dc5b9cbdd4b4deb587720ec1aeb59a020cf15002b9593eef/zipp-3.5.0-py3-none-any.whl
[STG.sup]         Collecting typing-extensions>=3.6.4; python_version < "3.8" (from importlib-metadata; python_version < "3.8"->click>=7.1.2->Flask>=0.12->-r /home/vcap/app/requirements.txt (line 1))
[STG.sup]           Downloading https://files.pythonhosted.org/packages/74/60/18783336cc7fcdd95dae91d73477830aa53f5d3181ae4fe20491d7fc3199/typing_extensions-3.10.0.2-py3-none-any.whl
[STG.sup]         Installing collected packages: zipp, typing-extensions, importlib-metadata, click, MarkupSafe, Jinja2, Werkzeug, itsdangerous, Flask
[STG.sup]         Successfully installed Flask-2.0.1 Jinja2-3.0.1 MarkupSafe-2.0.1 Werkzeug-2.0.1 click-8.0.1 importlib-metadata-4.8.1 itsdangerous-2.0.1 typing-extensions-3.10.0.2 zipp-3.5.0
[STG.sup]  .

[STG.sup]  

[STG.fin]  -----> Running go build finalize
[STG.fin]  /buildpacks/test-app/0 /buildpacks/test-app/0
[STG.fin]  /buildpacks/test-app/0
Buildpack #0 successfully applied
Successfully built 0c1529c67b3e
Successfully tagged test-app:latest

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
