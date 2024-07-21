# Copyright (c) 2024 AlertAvert.com.  All rights reserved.
# Created by M. Massenzio, 2022-03-14

# ANSI color codes
GREEN=$(shell tput -Txterm setaf 2)
YELLOW=$(shell tput -Txterm setaf 3)
RED=$(shell tput -Txterm setaf 1)
BLUE=$(shell tput -Txterm setaf 6)
RESET=$(shell tput -Txterm sgr0)

# Go platform management
GOOS ?= $(shell uname -s | tr "[:upper:]" "[:lower:]")
GOMOD := $(shell go list -m)

UNAME_M := $(shell uname -m)
ifeq ($(UNAME_M),x86_64)
    GOARCH = amd64
else ifeq ($(UNAME_M),aarch64)
    GOARCH = arm64
else ifeq ($(UNAME_M),armv6l)
    GOARCH = arm
else ifeq ($(UNAME_M),armv7l)
    GOARCH = arm
else ifeq ($(UNAME_M),armv8l)
    GOARCH = arm64
else
    $(error Unsupported architecture $(UNAME_M))
endif

yq != which yq
ifeq ($(strip $(yq)),)
  $(error yq not installed)
endif

settings != find . -name settings.yaml
ifeq ($(strip $(settings)),)
  $(warning $(YELLOW)This makefile requires a settings.yaml file to define appname and version$(RESET))
else
  appname != yq -r .name settings.yaml
  version != yq -r .version settings.yaml
endif

# Versioning
# The `version` is a static value, set in settings.yaml, and ONLY used to tag the release,
# `release` includes the git SHA and will be used to tag the binary and container.
git_commit != git rev-parse --short HEAD
ifndef version
  $(error $(RED)version must be defined, use yq and settings.yaml, or define it before including this file$(RESET))
endif
release := v$(version)-g$(git_commit)


# Certificates
certs_dir := ssl-config
ca-csr := $(certs_dir)/ca-csr.json
ca-config := $(certs_dir)/ca-config.json
server-csr := $(certs_dir)/localhost-csr.json

# Dockerfile
compose := docker/docker-compose.yaml
dockerfile := docker/Dockerfile

##@ Help
#
# The help target prints out all targets with their descriptions organized
# beneath their categories.
#
# The categories are represented by '##@' and the target descriptions by '##'.
#
# A category is defined if there's a line starting with ##@ <CATEGORY>,
# that gets pretty-printed as a category.
# A target is defined by a trailing comment starting with ##.
#
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
#
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php
#
.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

