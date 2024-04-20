# 1password-kubeconfig

A tool for managing your kubeconfig and delegating authentication to 1Password.


This leverages the built-in `exec` capability of the kubeconfig file to run the 1password CLI and authenticate with the Kubernetes cluster.
See https://kubernetes.io/docs/reference/access-authn-authz/authentication/#client-go-credential-plugins for more information.

## Usage

The command is designed to manage the kubeconfig file and then delegate authentication to the 1password CLI, so has two main commands:


```
1password-kubeconfig [command]

Available Commands:
auth        Use this command with a secret-name to authenticate.
update      Updates the kubeconfig file based on secrets from 1password.

Flags:
-h, -help   Show help for the 1password-kubeconfig command.

Use "1password-kubeconfig [command] -h" for more information about a command.
```

You will generally run `1password-kubeconfig update` to update your kubeconfig file with the latest configuration from 1password.

This configuration will then use `1password-kubeconfig auth` to authenticate with the cluster by requesting the client certificate and key from 1password.

## Configuring 1password

The tool expects the following fields to be present in the 1password item:

- `server`: The URL of the Kubernetes API server.
- `insecure-skip-tls-verify`: A boolean value to skip TLS verification.
- `certificate-authority`: The base64 encoded certificate authority data.
- `client-certificate`: The base64 encoded client certificate data.
- `client-key`: The base64 encoded client key data.


The first 3 fields will be injected into the kubeconfig file. The last 2 fields are used to by the `auth` command and will be returned to `kubectl`, when requested.

These secret items should have a tag of `kubeconfig` to be picked up by the tool. This tag can be customized with the `--tag` flag.
