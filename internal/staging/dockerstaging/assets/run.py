#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Program to run Cloudfoundry process inside cloudfoundry/cflinuxfs3 Docker container
"""
__program__ = "runner"
__version__ = "0.1.0"
__author__ = "Jose Riguera"
__year__ = "2021"
__email__ = "<jose.riguera@springer.com>"
__license__ = "MIT"
__purpose__ = """
Runner for CF applications created by staging.py.
"""

import sys
import os
import time
import argparse
import re
import yaml
import logging
import signal
import threading
import uuid
import socket
import json
import pwd

from subprocess import (Popen, PIPE)
from queue import Queue
from collections import deque
from pathlib import Path

####

class Runner(object):

    def __init__(self, working_path, env={}, user="", logger=None):
        self.working_path = working_path
        self.env = env
        self.queue = Queue()
        self.procs = deque()
        self.threads = []
        self.user = user
        if not logger:
            logger = logging.getLogger(self.__class__.__name__)
        self.logger = logger
        signal.signal(signal.SIGUSR1, self._progagate_signal)
        signal.signal(signal.SIGINT, self._progagate_signal)
        signal.signal(signal.SIGTERM, self._progagate_signal)        
        signal.siginterrupt(signal.SIGUSR1, False)

    def _set_user(self):
        if self.user != "":
            try:
                pw = pwd.getpwnam(self.user)
                uid = pw.pw_uid
            except:
                self.logger.error("User '%s' not found in the sytem" % (self.user))
                raise
            self.logger.debug("Setting running user: '%s'" % (self.user))
            def changeuser():
                os.setgid(uid)
                os.setuid(uid)
            return changeuser
        return lambda: None

    def _thread_runner(self, command, env={}, shell=False, wpath=None):
        environ = os.environ.copy()
        environment = {**self.env, **env}
        if environment:
            environ.update(environment)
        working_path = self.working_path
        if wpath != None:
            working_path = wpath
        kwargs = dict(
            cwd = working_path,
            shell = shell,
            env = environ,
            start_new_session = True,
            preexec_fn = self._set_user(),
        )
        this = threading.current_thread()
        proc = Popen(command, **kwargs)
        start = time.time()
        self.logger.debug("Running thread '%s' controlling pid %s: %s" % (this.name, proc.pid, command))
        self.procs.append(proc)
        rc = proc.wait()
        end = time.time()
        self.queue.put((this, proc, start, end, rc))

    def _progagate_signal(self, signum, frame):
        self.logger.info("Propagating signal '%s' to all children ..." % (signum))
        for p in self.procs:
            if p.poll() is None:
                #p.send_signal(signum)
                pgrp = os.getpgid(p.pid)
                self.logger.debug("Sending signal %s to process group %s" % (signum, pgrp))
                os.killpg(pgrp, signum)

    def run(self, exit_if_any=False):
        result = {}
        for thread in self.threads:
            thread.start()
        counter = len(self.threads)
        exited = False
        while counter > 0:
            # blocks until the item is available
            thread, proc, start, end, rc = self.queue.get()
            self.logger.debug("Thread %s running pid %s finished with returncode %s" % (thread.name, proc.pid, rc))
            self.queue.task_done()
            result[thread.name] = (proc.args, proc.pid, start, end, rc)
            thread.join()
            counter -= 1
            if exit_if_any and not exited:
                exited = True
                processes = [ p.pid for p in self.procs if p.poll() is None ]
                if processes:
                    self.logger.debug("Sending KILL signal to all processes: %s" % (processes))
                    self._progagate_signal(signal.SIGKILL, 0)
        self.threads = []
        self.queue = Queue()
        self.procs = deque()
        return result

    def task(self, name, command, env={}, shell=False, wpath=None):
        t = threading.Thread(name=name, target=self._thread_runner, args=(command, env, shell, wpath))
        t.daemon = True
        self.threads.append(t)



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
        "path": '.',
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



class CFRunner(object):

    def __init__(self, homedir, cfmanifest, user="", variables=None, logger=None):
        # homedir = /var/vcap
        # buildpacksdir = directory to download/process buildpacks
        # cachedir = directory used by buildpacks for caching their stuff
        # contextdir = directory where "cf push" runs
        # manifest = path to the CF manifest file
        if not logger:
            logger = logging.getLogger(self.__class__.__name__)
        self.logger = logger
        self.homedir = homedir
        self.user = user
        self.appdir = os.path.join(homedir, 'app')
        self.depsdir = os.path.join(homedir, 'deps')
        self.initd = os.path.join(homedir, 'init.d')
        cfmanifestpath = os.path.join(self.appdir, cfmanifest)
        self.logger.debug("Starting CF runner process: homedir=%s, appdir=%s, manifest=%s" % (homedir, self.appdir, cfmanifestpath))
        try:
            self.manifest = CFManifest(cfmanifestpath, variables, logger)
        except Exception as e:
            raise
        try:
            for app in self.manifest.list_apps():
                self.logger.debug("Found application %s in manifest file" % (app))
        except KeyError as e:
            self.logger.error("CloudFoundry manifest is incomplete: %s" % (str(e)))
            raise

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
        uris = os.getenv('APP_URIS', 'app.cf.local').split(',')
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
            instance_id = str(uuid.uuid5(uuid.NAMESPACE_DNS, app_name)),
            instance_index = '0',
            cf_api = os.getenv('CF_API', 'https://api.cf.local'),
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

    def get_default_running_vars(self, name, app_manifest):
        env_vars = {}
        default_running_vars = dict(
            MEMORY_LIMIT = app_manifest['memory'],
            PORT = os.getenv('APP_PORT', '8080'),
            DATABASE_URL = '',
            INSTANCE_INDEX = '0',
            INSTANCE_GUID = str(uuid.uuid5(uuid.NAMESPACE_DNS, name)),
            CF_INSTANCE_GUID = str(uuid.uuid5(uuid.NAMESPACE_DNS, name)),
            CF_INSTANCE_INDEX = '0',
            CF_INSTANCE_IP = self.get_internal_ip(),
            CF_INSTANCE_PORT = os.getenv('APP_PORT', '8080'),
            CF_INSTANCE_ADDR = self.get_internal_ip() + ':' + os.getenv('APP_PORT', '8080'),
            CF_INSTANCE_INTERNAL_IP = self.get_internal_ip(),
            CF_INSTANCE_PORTS = self.get_default_instance_ports(name, app_manifest),
            VCAP_APPLICATION = self.get_default_vcap_application(name, app_manifest),
            VCAP_PLATFORM_OPTIONS = '{}',
            VCAP_SERVICES = os.getenv('CF_VCAP_SERVICES', '{}'),
        )
        for k, v in default_running_vars.items():
            if k not in os.environ:
                self.logger.debug("Providing running environment variable: %s=%s" % (k, v))
                env_vars[k] = v
            else:
                self.logger.debug("Running environment variable already provided: %s=%s" % (k,  os.environ[k]))
                env_vars[k] = os.environ[k]
        return env_vars

    def get_keys_values_from_file(self, f):
        result = {}
        if os.path.isfile(f):
            with open(f) as af:
                for line in af:
                    name, var = line.partition("=")[::2]
                    result[name.strip()] = var.strip().strip('"')
        return result

    def get_value_from_file(self, f):
        result = ""
        with open(f) as af:
            result = af.read()
        return result

    def get_k8s_running_vars(self, name, app_manifest, k8s_cf_env):
        # Exported in k8s_cf_env using downward api
        # https://kubernetes.io/docs/tasks/inject-data-application/downward-api-volume-expose-pod-information/
        if not os.path.isdir(k8s_cf_env):
            return
        annotations = self.get_keys_values_from_file(os.path.join(k8s_cf_env, "annotations"))
        labels = self.get_keys_values_from_file(os.path.join(k8s_cf_env, "labels"))
        try:
            # Memory limit comes from API in M
            memory_limit = self.get_value_from_file(os.path.join(k8s_cf_env, "MEMORY_LIMIT"))
        except Exception as e:
            self.logger.error("Unable to read Downward-api file: %s. Falling back to default value." % e)
            # memory_limit = app_manifest['memory']
            memory_limit = "1024"
        try:
            cpu_limit = self.get_value_from_file(os.path.join(k8s_cf_env, "CPU_LIMIT"))
        except Exception as e:
            self.logger.error("Unable to read Downward-api file: %s. Falling back to 1 CPU." % e)
            cpu_limit = "1"
        try:
            uid = self.get_value_from_file(os.path.join(k8s_cf_env, "INSTANCE_GUID"))
        except Exception as e:
            self.logger.error("Unable to read Downward-api file: %s. Generating random UUID" % e)
            uid = str(uuid.uuid5(uuid.NAMESPACE_DNS, name))
        try:
            instance_index = labels["statefulset.kubernetes.io/pod-name"].rsplit("-", 1)[1]
        except Exception as e:
            self.logger.error("Unable calculate instance index: %s. Setting to 0" % e)
            instance_index = '0'
        env_vars = {}
        default_running_vars = dict(
            PORT = os.getenv('APP_PORT', '8080'),
            CPU_LIMIT = cpu_limit,
            MEMORY_LIMIT = memory_limit+"M",
            INSTANCE_INDEX = instance_index,
            INSTANCE_GUID = uid,
            CF_INSTANCE_GUID = uid,
            CF_INSTANCE_INDEX = instance_index,
            CF_INSTANCE_IP = self.get_internal_ip(),
            CF_INSTANCE_PORT = os.getenv('APP_PORT', '8080'),
            CF_INSTANCE_ADDR = self.get_internal_ip() + ':' + os.getenv('APP_PORT', '8080'),
            CF_INSTANCE_INTERNAL_IP = self.get_internal_ip(),
            CF_INSTANCE_PORTS = self.get_default_instance_ports(name, app_manifest),
        )
        app_name = os.getenv('APP_NAME', name)
        if not app_name:
            app_name = name
        uris = []
        for k, value in annotations.items():
            if k.startswith("kubefoundry/route"):
                uris.append(value)
        space = annotations.get("kubefoundry/space", os.getenv('CF_SPACE', 'null'))
        org = annotations.get("kubefoundry/org", os.getenv('CF_ORG', 'null'))
        vcap_app = dict(
            cf_api = os.getenv('CF_API', 'https://kubefoundry.local'),
            limits = {
                "fds": 16384,
                "mem": int(memory_limit) * 1048576,
                "disk": 4000 * 1048576,
            },
            users = 'null',
            name = app_name,
            instance_id = uid,
            instance_index = instance_index,
            application_name = app_name,
            application_id = annotations.get("kubefoundry/appuid.0", uid),
            version = annotations.get("kubefoundry/version.0", os.getenv('APP_VERSION', 'latest')),
            application_version = annotations.get("kubefoundry/version.0", os.getenv('APP_VERSION', 'latest')),
            uris = uris,
            application_uris = uris,
            space_name =  space,
            space_id = str(uuid.uuid5(uuid.NAMESPACE_DNS, space)),
            organization_name = org,
            organization_id = str(uuid.uuid5(uuid.NAMESPACE_DNS, org))
        )
        default_running_vars['VCAP_APPLICATION'] = json.dumps(vcap_app)
        for k, v in default_running_vars.items():
            if k not in os.environ:
                self.logger.debug("Providing running environment variable: %s=%s" % (k, v))
                env_vars[k] = v
            else:
                self.logger.debug("Running environment variable already provided: %s=%s" % (k,  os.environ[k]))
                env_vars[k] = os.environ[k]
        return env_vars


    def staging_info(self):
        staging_info_file = os.path.join(self.homedir, "staging_info.yml")
        self.logger.debug("Reading %s" % staging_info_file)
        try:
            with open(staging_info_file) as file:
                staging_info = yaml.load(file, Loader=yaml.SafeLoader)
        except IOError as e:
            self.logger.error("Cannot read CF staging_info.yml. %s" % (str(e)))
            raise
        return staging_info

    def run(self, exit_if_any=True, read_manifest_env=True, fake_cf_env=True, kubefoundry_env_path=''):
        runner = Runner(self.appdir, {}, self.user, self.logger)
        for f in Path(self.initd).glob('*.sh'):
            m = re.match(r"(\d+_\d+|\d+)_(.*)\.sh$", f.name)
            if m is not None:
                app_name = m.group(2)
                manifest = self.manifest.get_app_params(app_name)
                cfenv = {}
                if fake_cf_env:
                    self.logger.debug("Application running in local container, generating fake metadata ....")
                    cfenv = self.get_default_running_vars(app_name, manifest)
                if kubefoundry_env_path:
                    self.logger.debug("Application running in Kubernetes, trying to get metadata ....")
                    cfenv = self.get_k8s_running_vars(app_name, manifest, kubefoundry_env_path)
                manifestenv = {}
                if read_manifest_env:
                    manifestenv = manifest['env']
                cmd = [ str(f) ]
                if self.logger.level == logging.DEBUG:
                    cmd.append('--debug')
                env = {**cfenv, **manifestenv}
                runner.task(f.stem, cmd, env)
        output = runner.run(True)
        rcall = 0
        for name, result in output.items():
            cmd, pid, start, end, rc = result
            self.logger.info("Application %s (pid=%s) exited with code %s" % (name, pid, rc))
            rcall += rc
        return rcall


def main():
    logger = logging.getLogger()
    handler = logging.StreamHandler(sys.stdout)
    logger.addHandler(handler)
    # Argument parsing
    epilog = __purpose__ + '\n'
    epilog += __version__ + ', ' + __year__ + ' ' + __author__ + ' ' + __email__
    parser = argparse.ArgumentParser(formatter_class=argparse.RawTextHelpFormatter, description=__doc__, epilog=epilog)
    parser.add_argument('-d', '--debug', action='store_true', default=False, help='Enable debug mode')
    parser.add_argument('-e', '--manifest-env', action='store_true', default=False, help='Use manifest environment variables when app runs')
    parser.add_argument('-f', '--cf-fake-env', action='store_true', default=False, help='Simulate fake CF environment variables')
    parser.add_argument('-k', '--cf-k8s-env', metavar='/path/to/volume', help='Generate CF environment variables from K8S volume info')
    parser.add_argument('-m', '--manifest', metavar='manifest.yml', default="manifest.yml", help='CloudFoundry application manifest file')
    parser.add_argument('-u', '--user', default="vcap", help='Run applicaion(s) as this user')
    parser.add_argument('-v', '--manifest-vars', metavar='vars.yml', default="vars.yml", help='CloudFoundry variables file for manifest')
    parser.add_argument('-H', '--home', default="/home/vcap", help='Cloudfoundry VCAP home folder')
    args = parser.parse_args()
    debugvar = os.environ.get("DEBUG", '')
    if args.debug or debugvar:
        logger.setLevel(logging.DEBUG)
        handler.setLevel(logging.DEBUG)
    else:
        logger.setLevel(logging.INFO)
        handler.setLevel(logging.INFO)
    try:
        cfmanifest = os.environ.get("CF_MANIFEST", args.manifest)
        cfmanifest_vars = os.environ.get("CF_VARS", args.manifest_vars)
        runner = CFRunner(args.home, cfmanifest, args.user, cfmanifest_vars, logger)
        rc = runner.run(True, args.manifest_env, args.cf_fake_env, args.cf_k8s_env)
        sys.exit(rc)
    except Exception as e:
       print("ERROR: %s" % str(e), file=sys.stderr)
       sys.exit(1)


if __name__ == "__main__":
    main()