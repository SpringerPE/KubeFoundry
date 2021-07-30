KubeFoundry
===========

KubeVela/K8S + CloudFoundry client


Usage
=====


Configuration
-------------

See `config.yml`  and  `internal/config/config.go`


Development
===========

Golang 1.16 . There is a `Makefile` to manage the development actions, releases
and binaries. `make build` generates binaries for: `linux-amd64`, `linux-arm-6`,
`linux-arm-7` and `make deb` generates debian packages for `deb-amd64`, `deb-armhf`

Golang Modules
--------------

```
go mod init         # If you are not using git, type `go mod init $(basename `pwd`)`
go mod vendor       # if you have vendor/ folder, will automatically integrate
go build
```

This method creates a file called `go.mod` in your projects directory. You can
then build your project with `go build`.


Debian package
--------------

The usual call to build a binary package is `dpkg-buildpackage -us -uc`.
You might call debuild for other purposes, like `debuild clean` for instance.

```
# -us -uc skips package signing.
dpkg-buildpackage -rfakeroot -us -uc
```

Author
======

(c) 2021 Springer Nature, Engineering Enablement, Jose Riguera

Apache 2.0
