#!/usr/bin/env zsh
#
# Enable TLS
# Created by M. Massenzio, 2022-09-10

set -eu

CFSSL_VERSION="v1.6.2"

# Install CFSSL tooling
go install github.com/cloudflare/cfssl/cmd/...@${CFSSL_VERSION}


if [[ -z ${USR_LOCAL} ]];
then
    echo "ERROR: \$USR_LOCAL is not defined in your env"
    exit
fi

CONFIG_PATH=${USR_LOCAL}/certs/statemachine
mkdir -p ${CONFIG_PATH}
make gencert

echo "SUCCESS: All certs/keys stored in ${CONFIG_PATH}"
