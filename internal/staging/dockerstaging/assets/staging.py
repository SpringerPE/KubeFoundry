#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Program to run Cloudfoundry staging process inside cloudfoundry/cflinuxfs3 Docker container
"""
__program__ = "staging"
__version__ = "0.1.0"
__author__ = "Jose Riguera"
__year__ = "2021"
__email__ = "<jose.riguera@springer.com>"
__license__ = "MIT"
__purpose__ = """
The idea is simulating the Cloudfoundry staging process when a docker container based on
cloudfoundry/cflinuxfs3 is being built. This scripts reads the manifest file to find and
download the buildpacks and run its interface. If no buildpacks are defined, it dowloads
all known buildpacks and tries them one by one. This staging process supports manifest
with multiple applications and multi-buildpacks.
The output is the application packaged in a Docker image ready to run.
"""

import sys
import os
import time
import argparse
import re
import errno
import pty
import shlex
import yaml
import logging
import shutil
import zipfile
import json
import socket
import uuid
import tempfile

from select import select
from subprocess import (Popen, PIPE)
from collections import OrderedDict
from urllib.parse import urlsplit

# Ordered list of buildpacks and urls for automatic detection, pointing to master branch
BUILDPACKS= OrderedDict(
    staticfile_buildpack='https://github.com/cloudfoundry/staticfile-buildpack.git',
    java_buildpack='https://github.com/cloudfoundry/java-buildpack.git',
    python_buildpack='https://github.com/cloudfoundry/python-buildpack.git',
    ruby_buildpack='https://github.com/cloudfoundry/ruby-buildpack.git',
    nodejs_buildpack='https://github.com/cloudfoundry/nodejs-buildpack.git',
    php_buildpack='https://github.com/cloudfoundry/php-buildpack.git',
    go_buildpack='https://github.com/cloudfoundry/go-buildpack.git',
    dotnet_core_buildpack='https://github.com/cloudfoundry/dotnet-core-buildpack.git',
    binary_buildpack='https://github.com/cloudfoundry/binary-buildpack.git',
    nginx_buildpack='https://github.com/cloudfoundry/nginx-buildpack.git',
    r_buildpack='https://github.com/cloudfoundry/r-buildpack.git',
)


INIT_SCRIPT='''#!/bin/bash
# This file was automatically generated

# source all files 
load_folder() {
    local dir="${1}"
    local files=()
    if [ -d "${dir}" ]
    then
        # Get list of files by order in the specific path
        while IFS=  read -r -d $'\0' line
        do
            files+=("${line}")
        done < <(find -L ${dir}  -maxdepth 1 -type f -name '*.sh' -print0 | sort -z)
        # launch files
        for filename in "${files[@]}"
        do
            echo "Loading ${filename}"
            source "${filename}"
        done
    fi
}

export HOME="${HOME-/home/vcap/app}"
export LANG="${LANG-C.UTF-8}"
export USER="${USER-root}"
export TMPDIR="${TMPDIR-/home/vcap/tmp}"
export DEPS_DIR="${DEPS_DIR-/home/vcap/deps}"

case "$1" in
    --help|-h)
        echo "Script to start $0 application in the same way as CF"
        echo "Usage: $0 [--help|--debug]"
        exit 1
        ;; 
    --debug|-d)
        DEBUG=1
        ;;
esac

[ -z ${DEBUG} ] || set -x
load_folder "/home/vcap/profile.d"
load_folder "${HOME}/.profile.d"
[ -f "${HOME}/.profile" ] && source "${HOME}/.profile"
[ -z ${DEBUG} ] || env

'''


class Runner(object):
    ansi_escape = re.compile(r'(?:\x1B[@-_]|[\x80-\x9F])[0-?]*[ -/]*[@-~]')

    @classmethod
    def call(cls, command, env={}, working_path=None):
        if not working_path:
            working_path = os.getcwd()
        cmd = cls(working_path)
        return cmd.run(command, env, True, False)

    def __init__(self, working_path, env={}, logger=None):

        def _stdout_(*args, **kwargs):
            prefix = kwargs.pop('prefix', '')
            echo = kwargs.pop('echo', True)
            if self.logger:
                self.logger.debug(*args, **kwargs)
            if echo:
                predefined = dict(flush=True)
                new = {**predefined, **kwargs}
                if prefix: 
                    print(prefix, *args, **new)
                else:
                    print(*args, **new)
    
        def _stderr_(*args, **kwargs):
            prefix = kwargs.pop('prefix', '')
            echo = kwargs.pop('echo', True)
            if self.logger:
                self.logger.debug(*args, **kwargs)
            if echo:
                predefined = dict(file=sys.stderr, flush=True)
                new = {**predefined, **kwargs}
                if prefix: 
                    print(prefix, *args, **new)
                else:
                    print(*args, **new)

        self.__stderr__ = _stderr_
        self.__stdout__ = _stdout_
        self.logger = logger
        self.working_path = working_path
        self.env = env

    def set_stderr(self, fun):
        self.__stderr__ = fun

    def set_stdout(self, fun):
        self.__stdout__ = fun

    def run(self, command, env={}, shell=False, output_echo=True, output_prexif='', wpath=None):
        stdout = []
        stderr = []
        environ = os.environ.copy()
        environment = {**self.env, **env}
        if environment:
            environ.update(environment)
        working_path = wpath if wpath != None else self.working_path
        masters, slaves = zip(pty.openpty(), pty.openpty())
        kwargs = dict(
            cwd = working_path,
            shell = shell,
            env = environ,
            stdin = slaves[0],
            stdout = slaves[0],
            stderr = slaves[1],
        )
        if self.logger:
            self.logger.debug("Running: %s" % command)
        with Popen(command, **kwargs) as p:
            for fd in slaves:
                # no input
                os.close(fd)
            readable = {
                masters[0]: sys.stdout.buffer,
                masters[1]: sys.stderr.buffer,
            }
            stdout_buffer = b""
            stderr_buffer = b""
            while readable:
                done = False
                for fd in select(readable, [], [])[0]:
                    try:
                        data = os.read(fd, 1024)
                        # read available
                    except OSError as e:
                        if e.errno != errno.EIO:
                            if self.logger:
                                self.logger.error("Cannot read from PTY: %s" % (str(e)))
                            raise
                        # EIO means EOF on some systems
                        done = True
                        del readable[fd]
                    else:
                        if not data:
                            done = True
                            del readable[fd]
                        else:
                            lines = data.split(b"\n")
                            if fd == masters[0]:
                                # stdout
                                lines[0] = stdout_buffer + lines[0]
                                stdout_buffer = lines[-1]
                                stdout = stdout + lines[:-1]
                                for l in lines[:-1]:
                                    self.__stdout__(l.decode('utf-8'), echo=output_echo, prefix=output_prexif)
                            else:
                                # stderr
                                lines[0] = stdout_buffer + lines[0]
                                stderr_buffer = lines[-1]
                                stderr = stderr + lines[:-1]
                                for l in lines[:-1]:
                                    self.__stderr__(l.decode('utf-8'), echo=output_echo, prefix=output_prexif)
                            readable[fd].flush()
                    if done:
                        # add rest of the buffer to stdout and stderr
                        if fd == masters[0]:
                            if stdout_buffer:
                                stdout = stdout + [ stdout_buffer ]
                                self.__stdout__(stdout_buffer.decode('utf-8'), echo=output_echo, prefix=output_prexif)
                        else:
                            if stderr_buffer:
                                stderr = stderr + [ stderr_buffer ]
                                self.__stderr__(stderr_buffer.decode('utf-8'), echo=output_echo, prefix=output_prexif)
        for fd in masters:
            os.close(fd)
        # cleanup ansi colors and other stuff
        stdout = [self.ansi_escape.sub('', line.decode('utf-8').strip('\r')) for line in stdout]
        stderr = [self.ansi_escape.sub('', line.decode('utf-8').strip('\r')) for line in stderr]
        return p.returncode, stdout, stderr



class Git(object):
    @classmethod
    def download(cls, url, directory, tag=None, bare=True, echo=False):
        if os.path.isdir(directory):
            raise IsADirectoryError("Directory already exists")
        git = cls(directory, echo)
        rc, out, err = git.clone(["--recurse-submodules", url])
        if rc != 0:
            raise IOError(" ".join(err))
        if tag:
            checkout = []
            _, out, _ = git.tag(["--sort=-refname", "--list", tag])
            if out:
                checkout = ["tags/%s" % (out[0])]
            else:
                raise ValueError("Not found tag/branch: %s" % tag)
            rc, _, err = git.checkout(checkout)
            if rc != 0:
                raise ValueError("Checkout repository %s" % " ".join(err))
        if bare:
            git.clean()

    def __init__(self, directory, echo=True, logger=None):
        if not logger:
            logger = logging.getLogger(self.__class__.__name__)
        self.logger = logger
        if os.path.abspath(directory) == '/':
            msg = "Git directory cannot be root: %s" % directory
            self.logger.error(msg)
            raise ValueError(msg)
        self.directory = directory
        self.echo = echo
        env = dict(TERM='vt220')
        self.runner = Runner(directory, env, logger)

    def run(self, cmd, *args, **kwargs):
        args_string = list(args)
        return self._command_(cmd)(args_string, **kwargs)

    def clean(self):
        try:
            for f in [".git", ".gitignore", ".gitallowed"]:
                p = os.path.join(self.directory, f)
                if os.path.isdir(p):
                    shutil.rmtree(p)
                elif os.path.isfile(p):
                    os.remove(p)
        except OSError as e:
            self.logger.error("Error deleting %s: %s" % (e.filename, e.strerror))
            raise

    def __getattr__(self, key):
        return self._command_(key)

    def _command_(self, key):
        def proxy(cmd=[], **kwargs):
            args = ["git", key] + cmd
            path = self.directory
            if key == "clone":
                # when clone we go to parent folder
                path = os.path.abspath(os.path.join(self.directory, os.pardir))
                args.append(self.directory)
            return self.runner.run(args, {}, False, self.echo, "[GIT] ", path)
        return proxy



class CFManifest(object):
    AppParamsDefaults = {
        "buildpacks": [],
        "command": '',
        "disk_quota": '2048M',
        "docker": {},
        "health-check-http-endpoint": '/',
        "health-check-type": "port",
        "instances": 1,
        "memory": "1024M",
        "metadata": {},
        "no-route": False,
        "path": '',
        "processes": [],
        "random-route": False,
        "routes": [],
        "sidecars": [],
        "stack": 'cflinuxfs3',
        "timeout": 60,
        "env": {},
        "services": [],
    }

    def __init__(self,  manifest, variables=None, logger=None):
        if not logger:
            logger = logging.getLogger(self.__class__.__name__)
        self.logger = logger
        self.logger.debug("Reading Cloudfoundry manifest: %s" % manifest)
        try:
            with open(manifest) as file:
                self.manifest = yaml.load(file, Loader=yaml.SafeLoader)
        except IOError as e:
            self.logger.error("Cannot read CF manifest. %s" % (str(e)))
            raise
        if variables:
            self.logger.debug("Trying to read variables manifest: %s" % variables)
            try:
                with open(variables) as file:
                    self.variables = yaml.load(file, Loader=yaml.SafeLoader)
            except IOError as e:
                self.logger.debug("Skipping, not found %s" % (variables))
                self.variables = {}    
        else:
            self.variables = {}


    def get_version(self):
        try:
            return self.manifest['version']
        except:
            return 1

    def _interpolate(self, app, key):
        if not key in self.AppParamsDefaults.keys():
            raise ValueError("Key '%s' is unknown in CF manifest reference" % key)
        try:
            # quick and dirty alg, but works :-)
            result = app[key]
            for k, v  in self.variables.items():
                rpl = "((%s))" % k
                if type(result) is list:
                    new = []
                    for i in result:
                        if isinstance(i, str):
                            new.append(i.replace(rpl, str(v)))
                        elif isinstance(i, dict):
                            # routes [{route:blabla}]
                            for nk, nv in i.items():
                                if isinstance(nv, str):
                                    i[nk] = nv.replace(rpl, str(v))
                            new.append(i)
                        else:
                            new.append(i)
                    result = new
                elif type(result) is dict:
                    for nk, nv in result.item():
                        if isinstance(nv, str):
                            result[nk] = nv.replace(rpl, str(v))
                elif type(result) is str:
                    result = result.replace(rpl, str(v))
            return result
        except Exception as e:
            return self.AppParamsDefaults[key]

    def list_apps(self):
        result = []
        for app in self.manifest['applications']:
            result.append(app['name'])
        return result

    def get_app_params(self, name):
        for app in self.manifest['applications']:
            if app['name'] == name:
                params = {}
                for p in self.AppParamsDefaults.keys():
                    params[p] = self._interpolate(app, p)
                return params
        raise ValueError("Application '%s' not found in manifest" % name)


class Buildpack(object):
    def __init__(self,  name, index, dir, appdir, depsdir, cachedir, env={}, logger=None):
        if not logger:
            logger = logging.getLogger(self.__class__.__name__)
        self.logger = logger
        self.name = name
        self.index = index
        self.dir = dir
        self.appdir = appdir
        self.cachedir = cachedir
        self.depsdir = depsdir
        self.runner = Runner(dir, env, logger)

    def detect(self, echo=True, env={}):
        self.logger.debug("Buildpack #%s running detect step ... " % (self.index))
        cmd = [os.path.join(self.dir, "bin", "detect"), self.appdir]
        try:
            rc, _, _ = self.runner.run(cmd, env, False, echo, "[STG.det] ")
        except Exception as e:
            self.logger.error("Buildpack #%s, exception running detect: %s" % (self.index, str(e)))
            return False
        self.logger.info("Buildpack #%s detection: %s" % (self.index, rc == 0))
        return rc == 0

    def compile(self, echo=True, env={}):
        self.logger.debug("Buildpack #%s running compile step ... " % (self.index))
        try:
            os.makedirs(self.depsdir, mode=0o755, exist_ok=True)
        except OSError as e:
            self.logger.error("Directory cannot be created: %s" %  (str(e)))
            raise
        cmd = [os.path.join(self.dir, "bin", "compile"), self.appdir, self.cachedir]
        rc = 1
        try:
            rc, _, _ = self.runner.run(cmd, env, False, echo, "[STG.com] ")
        except Exception as e:
            self.logger.error("Buildpack #%s, exception running compile: %s" % (self.index, str(e)))
        return rc

    def supply(self, echo=True, env={}):
        self.logger.debug("Buildpack #%s running supply step ... " % (self.index))
        try:
            path = os.path.join(self.depsdir, str(self.index))
            os.makedirs(path, mode=0o755, exist_ok=True)
        except OSError as e:
            self.logger.error("Directory cannot be created: %s" %  (str(e)))
            raise
        cmd = [os.path.join(self.dir, "bin", "supply"), self.appdir, self.cachedir, self.depsdir, str(self.index)]
        rc = 1
        try:
            rc, _, _ = self.runner.run(cmd, env, False, echo, "[STG.sup] ")
        except Exception as e:
            self.logger.error("Buildpack #%s, exception running supply: %s" % (self.index, str(e)))
        return rc

    def finalize(self, echo=True, env={}):
        self.logger.debug("Buildpack #%s running finalize step ... " % (self.index))
        cmd = [os.path.join(self.dir, "bin", "finalize"), self.appdir, self.cachedir, self.depsdir, str(self.index)]
        rc = 1
        try:
            rc, _, _ = self.runner.run(cmd, env, False, echo, "[STG.fin] ")
        except Exception as e:
            self.logger.error("Buildpack #%s, exception running finalize: %s" % (self.index, str(e)))
        return rc

    def release(self, echo=True, env={}):
        self.logger.debug("Buildpack #%s running release step ... " % (self.index))
        cmd = [os.path.join(self.dir, "bin", "release"), self.appdir]
        config_vars = {}
        addons = []
        default_process_types = {}
        try:
            rc, out, _ = self.runner.run(cmd, env, False, echo, "[STG.rel] ")
        except Exception as e:
            self.logger.error("Buildpack #%s, exception running release: %s" % (self.index, str(e)))
            return False, default_process_types, config_vars, addons
        else:
            data = yaml.load("\n".join(out), Loader=yaml.SafeLoader)
            addons = data.get('addons', addons)
            if addons is None:
                addons = [] 
            config_vars = data.get('config_vars', config_vars)
            if config_vars is None:
                config_vars = {}
            default_process_types = data.get('default_process_types', default_process_types)
            if default_process_types is None:
                default_process_types = {}
            if len(default_process_types) > 0:
                self.logger.debug("Buildpack #%s provides startup command: %s" % (self.index, default_process_types))
            else:
                self.logger.debug("Buildpack #%s does not provide startup command!" % (self.index))
            return True, default_process_types, config_vars, addons


    def run(self, detect=False, final=True, env={}, showall=None):
        default_process_types = {}
        config_vars = {}
        addons = []
        echo_detect_release = True
        echo_supply_finalize = True
        if not showall:
            echo_detect_release = False
            echo_supply_finalize = True
        self.logger.info("Running staging process with buildpack #%s: %s" % (self.index, self.name))
        # https://docs.cloudfoundry.org/buildpacks/understand-buildpacks.html
        if detect:
            detect = self.detect(echo_detect_release, env)
        else:
            detect = True
        if detect:
            if not final:
                rc = self.supply(echo_supply_finalize, env)
                if rc != 0:
                    msg = "Error running supply step in buildpack #%s" % (self.index)
                    self.logger.error(msg)
                    raise ValueError(msg)
                self.logger.info("Non final buildpack #%s, skipping rest of steps" % (self.index))
            else:            
                if os.path.isfile(os.path.join(self.dir, "bin", "finalize")):
                    if os.path.isfile(os.path.join(self.dir, "bin", "supply")):
                        rc = self.supply(echo_supply_finalize, env)
                        if rc != 0:
                            msg = "Error running supply step in buildpack #%s" % (self.index)
                            self.logger.error(msg)
                            raise ValueError(msg)
                    rc = self.finalize(echo_supply_finalize, env)
                    if rc != 0:
                        msg = "Error running finalize step in buildpack #%s" % (self.index)
                        self.logger.error(msg)
                        raise ValueError(msg)
                else:
                    rc = self.compile(echo_supply_finalize, env)
                    if rc != 0:
                        msg = "Error running compile step in buildpack #%s" % (self.index)
                        self.logger.error(msg)
                        raise ValueError(msg)
                rc, default_process_types, config_vars, addons = self.release(echo_detect_release, env)
                if not rc:
                    msg = "Error running release step in buildpack #%s" % (self.index)
                    self.logger.error(msg)
                    raise ValueError(msg)
            self.logger.debug("Buildpack #%s successfully applied" % (self.index))
            return True, default_process_types, config_vars, addons
        else:
            self.logger.info("Skipping #%s buildpack!" % (self.index))
            return False, default_process_types, config_vars, addons



class CFStaging(object):
    Buildpacks = BUILDPACKS

    def __init__(self, homedir, buildpacksdir, cachedir, contextdir, healthcheck=None, logger=None):
        # homedir = /var/vcap
        # buildpacksdir = directory to download/process buildpacks
        # cachedir = directory used by buildpacks for caching their stuff
        # contextdir = directory where "cf push" runs
        # manifest = path to the CF manifest file
        if not logger:
            logger = logging.getLogger(self.__class__.__name__)
        self.logger = logger
        self.logger.debug("Starting staging process: homedir=%s, buildpacksdir=%s, cachedir=%s" % (homedir, buildpacksdir, cachedir))
        if os.path.abspath(buildpacksdir) == '/':
            msg = "Buildpack cannot be root: %s" % buildpacksdir
            self.logger.error(msg)
            raise ValueError(msg)
        try:
            os.makedirs(cachedir, mode=0o755, exist_ok=True)
            self.logger.debug("Directory for buildpacks caching '%s' created successfully" % cachedir)
        except OSError as e:
            self.logger.error("Buildpacks caching directory cannot be created: %s" % (str(e)))
            raise
        self.cachedir = cachedir
        for dir in ["app", "deps", "logs", "tmp", "init.d"]:
            path = os.path.join(homedir, dir)
            try:
                os.makedirs(path, mode=0o755, exist_ok=True)
                self.logger.debug("Directory '%s' created successfully" % path)
            except OSError as e:
                self.logger.error("Directory '%s' cannot be created: %s" % (path, str(e)))
                raise
        self.homedir = homedir
        self.contextdir = contextdir
        self.appdir = os.path.join(homedir, 'app')
        self.healthcheck = healthcheck
        self.depsdir = os.path.join(homedir, 'deps')
        self.initd = os.path.join(homedir, 'init.d')
        self.buildpacksdir = buildpacksdir
        self.cleaning_paths = []
        self.manifest = None

    def get_internal_ip(self):
        s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        try:
            # doesn't even have to be reachable
            s.connect(('1.1.1.1', 53))
            ip = s.getsockname()[0]
        except Exception:
            ip = '127.0.0.1'
        finally:
            s.close()
        return ip

    def get_default_vcap_application(self, name, app_manifest):
        uris = os.getenv('APP_URIS', '').split(',')
        # remove empty strings
        uris = [i for i in uris if i]
        for r in app_manifest['routes']:
            try:
                uris.append(r['route'])
            except:
                pass
        app_name = os.getenv('APP_NAME', name)
        if not app_name:
            app_name = name
        vcap_app = dict(
            cf_api = os.getenv('CF_API', 'https://api.cf'),
            limits = {
                "fds": 16384,
                "mem": app_manifest['memory'],
                "disk": app_manifest['disk_quota']
            },
            users = 'null',
            name = app_name,
            application_name = app_name,
            application_id = str(uuid.uuid5(uuid.NAMESPACE_DNS, app_name)),
            version = os.getenv('APP_VERSION', 'latest'),
            application_version = os.getenv('APP_VERSION', 'latest'),
            uris = uris,
            application_uris = uris,
            space_name = os.getenv('CF_SPACE', 'null'),
            space_id = str(uuid.uuid5(uuid.NAMESPACE_DNS, os.getenv('CF_SPACE', 'null'))),
            organization_id = os.getenv('CF_ORG', 'null'),
            organization_name = str(uuid.uuid5(uuid.NAMESPACE_DNS, os.getenv('CF_ORG', 'null')))
        )
        return json.dumps(vcap_app)

    def get_default_instance_ports(self, name, app_manifest):
        try:
            port = int(os.getenv('APP_PORT', '8080'))
        except:
            port = 8080
        instance_ports = [
            {
                "external": 80,
                "internal": port,
            },
        ]
        return json.dumps(instance_ports)

    def get_staging_vars(self, name, app_manifest):
        env_vars = {}
        default_staging_vars = dict(
            MEMORY_LIMIT = app_manifest['memory'],
            LANG = "en_US.UTF-8",
            CF_INSTANCE_INDEX = '0',
            CF_INSTANCE_IP = self.get_internal_ip(),
            CF_INSTANCE_PORT = os.getenv('APP_PORT', '8080'),
            CF_INSTANCE_ADDR = self.get_internal_ip() + ':' + os.getenv('APP_PORT', '8080'),
            CF_INSTANCE_INTERNAL_IP = self.get_internal_ip(),
            CF_INSTANCE_PORTS = self.get_default_instance_ports(name, app_manifest),
            CF_STACK = app_manifest['stack'],
            VCAP_APPLICATION = self.get_default_vcap_application(name, app_manifest),
            VCAP_PLATFORM_OPTIONS = '{}',
            VCAP_SERVICES = os.getenv('CF_VCAP_SERVICES', '{}'),
        )
        for k, v in default_staging_vars.items():
            if k not in os.environ:
                self.logger.debug("Defining staging environment variable: %s=%s" % (k, v))
                env_vars[k] = v
            else:
                self.logger.debug("Staging environment variable already defined: %s=%s" % (k,  os.environ[k]))
                env_vars[k] = os.environ[k]
        return env_vars

    def download_buildpack(self, name, path, force=False):
        if os.path.isdir(path):
            self.logger.debug("Buildpack '%s' already downloaded in '%s'" % (name, path))
            if not force:
                return False
            else:
                try:
                    shutil.rmtree(path)
                except OSError as e:
                    self.logger.error("Error deleting buildpack directory '%s': %s" % (e.filename, e.strerror))
                    raise
        self.cleaning_paths.append(path)
        scheme, netloc, urlpath, _, version = urlsplit(name)
        if not scheme:
            if urlpath in list(self.Buildpacks.keys()):
                scheme, netloc, urlpath, _, v = urlsplit(self.Buildpacks[urlpath])
                if not version:
                    version = v
            else:
                msg = "Unknown buildpack '%s', is not a git resource, neither is in the internal buildpack list" % name
                self.logger.error(msg)
                raise ValueError(msg)
        url = scheme + '://' + netloc + urlpath
        if not url.endswith('.git'):
            msg = "Unknown buildpack '%s', is not a git resource" % name
            self.logger.error(msg)
            raise ValueError(msg)
        self.logger.info("Downloading buildpack '%s' (%s) ..." % (name, url))
        Git.download(url, path, version, True, (self.logger.level == logging.DEBUG))
        self.logger.debug("Buildpack '%s' dowloaded to '%s'" % (name, path))
        return True

    def link_context(self):
        try:
            self.logger.debug("Deleting context directory: %s" % (self.contextdir))
            shutil.rmtree(self.contextdir)
            self.logger.debug("Creating link '%s' to '%s'" % (self.contextdir, self.appdir))
            os.symlink(self.appdir, self.contextdir, target_is_directory=True)
        except OSError as e:
            self.logger.error("Error deleting context directory '%s': %s" % (e.filename, e.strerror))
            raise

    def cleanup_buildpacks(self, also_cache=True):
        deleted = []
        self.logger.info("Deleting downloaded buildpacks ...")
        for path in self.cleaning_paths:
            if os.path.isdir(path):
                self.logger.debug("Deleting buildpack: %s" % (path))
                try:
                    shutil.rmtree(path)
                    deleted.append(path)
                except OSError as e:
                    self.logger.error("Error deleting buildpack directory '%s': %s" % (e.filename, e.strerror))
                    raise
        self.cleaning_paths = []
        if also_cache:
            try:
                for f in os.listdir(self.cachedir):
                    path = os.path.join(self.cachedir, f)
                    if os.path.isfile(path) or os.path.islink(path):
                        os.unlink(path)
                    else:
                        shutil.rmtree(path)
                    deleted.append(path)
            except OSError as e:
                self.logger.error("Error deleting cahce directory '%s': %s" % (e.filename, e.strerror))
                raise
        return deleted

    def _recursive_overwrite(self, src, dest, ignore=None):
        if os.path.isdir(src):
            if not os.path.isdir(dest):
                os.makedirs(dest)
            files = os.listdir(src)
            if ignore is not None:
                ignored = ignore(src, files)
            else:
                ignored = set()
            for f in files:
                if f not in ignored:
                    self._recursive_overwrite(os.path.join(src, f), os.path.join(dest, f), ignore)
        else:
            try: 
                shutil.copy2(src, dest)
            except shutil.SameFileError:
                src.replace(dest)
            #shutil.copyfile(src, dest)

    def append_procfile_commands(self, startcommands=[], sidecarcommands=[]):
        procfile = os.path.join(self.appdir, "Procfile")
        if not os.path.isfile(procfile):
            procfile = os.path.join(self.appdir, "procfile")
        if not os.path.isfile(procfile):
            self.logger.debug("No procfile found")
            return startcommands, sidecarcommands
        self.logger.debug("Reading %s" % procfile)
        with open(procfile, 'r') as f:
            for line in f:
                line = line.strip()
                if line.startswith('web:'):
                    cmd = line.split(':', 1)[1]
                    startcommands.append(cmd.strip())
                elif line.startswith('worker:'):
                    # WARNING: This is not correct, it should be in a different container!
                    cmd = line.split(':', 1)[1]
                    sidecarcommands.append(cmd.strip())
                # TODO: tasks
        return startcommands, sidecarcommands

    def append_manifest_commands(self, app_manifest, startcommands=[], sidecarcommands=[]):
        for sidecar in app_manifest['sidecars']:
            try:
                # prepend
                cmd = sidecar['command']
                sidecarcommand.append(cmd.strip())
            except:
                self.logger.error("Sidecar '%s' without 'command' key" % sidecar)
        if app_manifest['command']:
            # prepend
            startcommand.append(app_manifest['command'])
        return startcommands, sidecarcommands


    def _get_apps_buildpacks(self, appbits, application="", extra_buildpacks=[], force_download=False):
        # get list of apps from buildpack directory, first level is app name and
        # second level is a set of indexes representing the list buildpaks of the app 
        # buildpacks
        #     |- app-name1
        #     |     |
        #     |     |- 0 [python-buildpack]
        #     |     |- 1 [java-buildpack] (final)
        #     |- app-name2
        #     |     |- 0 [ruby-buildpack] (final)
        #     |- app-name3
        #           |- 0 [python-buildpack] 
        #           |- 1 [custom-buildpack] (final, folder exist, it will not be downloaded)
        #           ...
        # 
        app_settings = OrderedDict()
        for app_name in self.manifest.list_apps():
            app = self.manifest.get_app_params(app_name)
            if application:
                if app_name != application:
                    self.logger.info("Ignoring application name '%s' defined in the manifest" % application)
                    continue
            autodetect = False
            env = {}
            startcmd = ""
            manifest_buildpacks = extra_buildpacks
            try:
                manifest_buildpacks = manifest_buildpacks + app['buildpacks']
            except KeyError:
                msg = "Application name '%s' without buildpacks defined in the manifest" % app_name
                self.logger.info(msg)
            if not manifest_buildpacks:
                msg = "No buildpacks defined for application '%s', trying to autodetect a suitable one ...." % app_name
                self.logger.info(msg)
                # Assign all buildpacks to enable detection
                manifest_buildpacks = list(self.Buildpacks.values())
                autodetect = True
            if 'env' in app:
                env = app['env']
            if 'path' in app:
                if app['path']:
                    appbits = app['path']
            try:
                appdir = os.path.join(self.contextdir, appbits)
                if os.path.isfile(appdir):
                    # it has to be a zip file (like a jar), but it will accept other formats ;-)
                    # shutil.unpack_archive(appdir, self.appdir)
                    with zipfile.ZipFile(appdir, 'r') as zipapp:
                        contents = zipapp.namelist()
                        # Assuming the first entry in the zip file is the root path
                        base_path = contents[0]
                        remove_base_path = True
                        for item in contents:
                            if not item.startswith(base_path):
                                # zip file is the app itself because there are more that one
                                # file/folder in the root path
                                remove_base_path = False
                                break
                        if not remove_base_path:
                            zipapp.extractall(self.appdir)
                        else:
                            temp_path = tempfile.mkdtemp(dir=self.contextdir)
                            zipapp.extractall(temp_path)
                            self._recursive_overwrite(os.path.join(temp_path, base_path), self.appdir)
                            shutil.rmtree(temp_path)
                elif os.path.isdir(appdir):
                    #dst = os.path.join(self.appdir, app['path'])
                    # TODO: ignore also .cfignore
                    #shutil.copytree(appdir, self.appdir, ignore=shutil.ignore_patterns('.git'))
                    self._recursive_overwrite(appdir, self.appdir)
                else:
                    raise ValueError("application path not found: %s" % appbits)
            except Exception as e:
                self.logger.error("Cannot copy application files, %s" % str(e))
                raise
            appdir = self.appdir
            # List of buildpacks
            self.logger.info("Application '%s' buildpacks: %s" % (app_name, manifest_buildpacks))
            buildpacks = []
            try:
                for i in range(len(manifest_buildpacks)):
                    path = os.path.join(self.buildpacksdir, app_name, str(i))
                    bp_name = manifest_buildpacks[i]
                    self.download_buildpack(bp_name, path, force_download)
                    buildpacks.append(Buildpack(bp_name, i, path, appdir, self.depsdir, self.cachedir, env, self.logger))
                app_settings[app_name] = (autodetect, buildpacks, app)
            except Exception:
                self.logger.error("Errors processing buildpacks for application '%s'. Skipping application" % app_name)
        return app_settings

    def run(self, appbits, cfmanifest, application="", variables=None, extra_buildpacks=[], force=False):
        cfmanifestpath = os.path.join(self.contextdir, cfmanifest)
        try:
            self.manifest = CFManifest(cfmanifestpath, variables, self.logger)
            # Copy manifest to appdir
            path = os.path.join(self.appdir, cfmanifest)
            cfmanifestpath = shutil.copy(cfmanifestpath, path)
        except PermissionError as e:
            self.logger.error("Cannot copy CF manifest. %s" % (str(e)))
            raise
        try:
            for app in self.manifest.list_apps():
                self.logger.debug("Found application %s in manifest file" % (app))
                bpath = os.path.join(self.buildpacksdir, app)
                os.makedirs(bpath, mode=0o755, exist_ok=True)
                self.logger.debug("Directory for buildpacks '%s' created successfully" % bpath)
        except OSError as e:
            self.logger.error("Buildpacks directory cannot be created: %s" % (str(e)))
            raise
        except KeyError as e:
            self.logger.error("CloudFoundry manifest is incomplete: %s" % (str(e)))
            raise
        startcommands = OrderedDict()
        healthchecks = OrderedDict()
        app_settings = self._get_apps_buildpacks(appbits, application, extra_buildpacks, force)
        app_index = 0
        for app, settings in app_settings.items():
            autodetect = settings[0]
            buildpacks = settings[1]
            manifest = settings[2]
            amount = len(buildpacks)
            index = 1
            final_buildpack = "-"
            running_env = {}
            staging_env = self.get_staging_vars(app, manifest)
            startcommand, sidecarcommand = self.append_manifest_commands(manifest)
            startcommand, sidecarcommand = self.append_procfile_commands(startcommand, sidecarcommand)
            for buildpack in buildpacks:
                final = (index == amount) or autodetect
                try:
                    show_all_stages = self.logger.level == logging.DEBUG
                    applied, commands, newenvs, addons = buildpack.run(autodetect, final, staging_env, show_all_stages)
                    if applied:
                        final_buildpack = buildpack.name
                        if final and commands:
                            if 'web' in commands:
                                startcommand.append(commands['web'])
                            # TODO: addons, tasks
                        staging_env.update(newenvs)
                        running_env.update(newenvs)
                        if autodetect:
                            break
                except Exception as e:
                    self.logger.error("Cannot apply buildpack '%s' to application '%s'" % (buildpack.name, app))
                    raise
                index += 1
            self.logger.info("Application '%s' successfully staged/compiled" % (app))
            if startcommand:
                # Write staging_info.yml with the first startcommand of the list
                with open(os.path.join(self.homedir, 'staging_info.yml'), 'w') as staging_info:
                    info = {
                        "detected_buildpack": final_buildpack,
                        "start_command": startcommand[0]
                    }
                    staging_info.write(json.dumps(info))
                self._write_init(app, app_index, startcommand[0], running_env)
                kind = manifest.get('health-check-type', 'http')
                data = manifest.get('health-check-http-endpoint', '/')
                if kind == 'process':
                    data = startcommand[0]
                healthchecks[app] = (kind, data)
                startcommands[app] = startcommand
            index = 0
            for sd in sidecarcommand:
                # format is  0_0_app-name.sh
                self._write_init(str(index) + "_" + app, app_index, sd, running_env)
                index += 1
            app_index += 1
        self._write_healthcheck(healthchecks)
        return startcommands, healthchecks

    def _write_init(self, app, index, command, env={}):
        startup = os.path.join(self.initd, str(index) + '_' + app + '.sh')
        try:
            with open(startup, 'w') as w:
                print(INIT_SCRIPT, file=w)                
                print("cd %s\n" % (self.appdir), file=w)
                for k, v in env.items():
                    print("export %s=\"${%s-%s}\"" % (k, k, v.replace('"', '\\"').replace('\n', '\\n')), file=w)
                print("\n%s" % command, file=w)
            os.chmod(startup, 0o775)
        except OSError as e:
            self.logger.error("Startup file '%s' cannot be created: %s" % (startup, str(e)))
            raise
        self.logger.info("Application '%s' startup command: \033[0;33m%s\033[0m" % (app, command))


    def _write_healthcheck(self, app_healthchecks):
        if self.healthcheck:
            try:
                with open(self.healthcheck, 'w') as w:
                    print("#!/bin/bash -e", file=w)
                    print("# This file was generated by %s\n" % (os.path.basename(__file__)), file=w)
                    for app, healthcheck in app_healthchecks.items():
                        kind = healthcheck[0]
                        data = healthcheck[1]
                        print("# checks for %s" % (app), file=w)
                        if kind == "http":
                            print("curl --silent --fail --connect-timeout 2 http://127.0.0.1:${APP_PORT:-${PORT:-8080}}%s" % (data), file=w)
                        elif kind == "port":
                            print("nc -z -w 2 127.0.0.1 ${APP_PORT:-${PORT:-8080}}", file=w)
                        elif kind == "process":
                            print("pgrep --ignore-case --full %s >/dev/null" % (data), file=w)
                        else:
                            msg = "Process type '%s' not supported" % (kind)
                            self.logger.error(msg)
                            raise ValueError(msg)
                os.chmod(self.healthcheck, 0o775)
            except OSError as e:
                self.logger.error("Healthcheck file '%s' cannot be created: %s" % (self.healthcheck, str(e)))
                raise



def main():
    logger = logging.getLogger()
    handler = logging.StreamHandler(sys.stdout)
    logger.addHandler(handler)
    # Argument parsing
    epilog = __purpose__ + '\n'
    epilog += __version__ + ', ' + __year__ + ' ' + __author__ + ' ' + __email__
    parser = argparse.ArgumentParser(formatter_class=argparse.RawTextHelpFormatter, description=__doc__, epilog=epilog)
    parser.add_argument('-d', '--debug', action='store_true', default=False, help='Enable debug mode')
    parser.add_argument('-f', '--force', action='store_true', default=False, help='Force downloading buildpacks data')
    parser.add_argument('-b', '--buildpack', action='append', default=[], help='Buildpacks urls for staging the application')
    parser.add_argument('--builddir', default='/buildpacks' , help='Working directory for buildpacks')  
    parser.add_argument('--buildcache', default="/var/local/buildpacks/cache", help='Buildpacks cache directory')
    parser.add_argument('-m', '--manifest', default="manifest.yml", help='CloudFoundry application manifest file')
    parser.add_argument('-v', '--manifest-vars', default="vars.yml", help='CloudFoundry variables file for manifest')
    parser.add_argument('--home', default="/home/vcap", help='Cloudfoundry VCAP home folder')
    parser.add_argument('-a', '--app', default="", help='Application name (when a manifest has multiple applications)')
    parser.add_argument('--appcontext', default="/app", help='Application context folder within the container')
    parser.add_argument('--healthcheck', default="/healthcheck.sh", help='File to write the healthchecks')
    parser.add_argument('--link-context', action='store_true', default=False, help='Delete context folder and create a symlink to home/app')
    parser.add_argument('--clean', action='count', default=0, help='Delete the downloaded buildpacks, twice deletes also cache')
    parser.add_argument('application', default='.', type=str, help='Application zip file or directory')  
    args = parser.parse_args()
    if args.debug:
        logger.setLevel(logging.DEBUG)
        handler.setLevel(logging.DEBUG)
    else:
        logger.setLevel(logging.INFO)
        handler.setLevel(logging.INFO)
    try:
        sys.stdout.flush()
        sys.stderr.flush()
        cfmanifest = os.environ.get("CF_MANIFEST", args.manifest)
        cfmanifest_vars = os.environ.get("CF_VARS", args.manifest_vars)
        stage = CFStaging(args.home, args.builddir, args.buildcache, args.appcontext, args.healthcheck, logger)
        stage.run(args.application, cfmanifest, args.app, cfmanifest_vars, args.buildpack, args.force)
        if args.clean > 0:
            stage.cleanup_buildpacks((args.clean > 1))
        if args.link_context:
            stage.link_context()
        sys.exit(0)
    except Exception as e:
        print("ERROR: %s" % str(e), file=sys.stderr, flush=True)
        sys.exit(1)
    finally:
        sys.stdout.flush()
        sys.stderr.flush()
        time.sleep(1)

if __name__ == "__main__":
    main()