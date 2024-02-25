package config

const (
	// TlsConfigDirEnv is the env var which defines where are the certs and keys stored.
	TlsConfigDirEnv = "TLS_CONFIG_DIR"

	// DefaultConfigDir is the default directory for the key material
	// if TLS is enabled, but TlsConfigDirEnv is not defined,
	DefaultConfigDir = "/etc/statemachine/certs"

	// CAFile is the name of Certificate for the root CA (can be self-signed, in which
	// case it should be provided to the client too), use `make gencert` to generate.
	CAFile = "ca.pem"

	// ServerCertFile is the Server certificate (containing the valid host names), use `make gencert` to generate.
	ServerCertFile = "server.pem"

	// ServerKeyFile the private signing key for the certificate, use `make gencert` to generate.
	ServerKeyFile = "server-key.pem"
)
