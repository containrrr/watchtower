## Prerequisites
To contribute code changes to this project you will need the following development kits.
 * Go. [Download and install](https://golang.org/doc/install) the Go programming language
 * [docker](https://docs.docker.com/engine/installation/)

## Checking out the code
When cloning watchtower to your development environment you should place your forked repo within the [standard go code structure](https://golang.org/doc/code.html#Organization).
```bash
cd $GOPATH/src
mkdir <yourfork>
cd <yourfork>
git clone git@github.com:<yourfork>/watchtower.git
cd watchtower
```

## Building and testing
watchtower is a go application and is built with go commands. The following commands assume that you are at the root level of your repo.
```bash
go get -u github.com/Masterminds/glide # installs glide for vendoring
glide install                          # retrieves package dependencies
go build                               # compiles and packages an executable binary, watchtower
go test                                # runs tests
./watchtower                           # runs the application (outside of a container)
```
