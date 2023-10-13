# Enabling TLS

**Copyright (c) 2022 AlertAvert.com. All rights reserved**<br>
*Author: M. Massenzio, 2022-09-10*

**WARNING --- THIS IS STILL WIP AND MAY CHANGE SIGNIFICANTLY UNTIL IT BECOMES STABLE**


# Overview

We are using [Cloudflare CFSSL](https://github.com/cloudflare/cfssl): 
the installation and running is kept in the [`ssl-config/enable-ssl.sh`](ssl/enable-ssl.sh) script.

# Generating Server and CA Certs

Using the `gencert` Makefile action, the resulting secrets are generated in the `certs/` folder:

    make gencert

These will also be used in generating the Docker container, and copied in the default directory:

```golang
DefaultConfigDir = "/etc/statemachine/certs"
```

To change the location of where the server will be looking for secrets and certificates at startup use the `CONFIG_DIR` env var.

## Disabling TLS

Use the `DISABLE_TLS` env var (set to anything other than an empty string) to disable, both for client and server.
