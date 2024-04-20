package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

type ConfigSecret struct {
	Id                    string `json:"id,omitempty"`
	Name                  string `json:"name,omitempty"`
	Server                string `json:"server,omitempty"`
	InsecureSkipTLSVerify bool   `json:"insecure_skip_tls_verify,omitempty"`
	CertificateAuthority  string `json:"certificate_authority,omitempty"`
	ClientCertificate     string `json:"client_certificate,omitempty"`
	ClientKey             string `json:"client_key,omitempty"`
}

// ConfigName returns a slugified version of the secret name with the suffix ":op"
// This will allow you to recognize the config items that are managed by 1password.
func (s ConfigSecret) ConfigName() string {

	return fmt.Sprintf("%s:op", slugify(s.Name))
}

type Secret struct {
	Id      string        `json:"id,omitempty"`
	Title   string        `json:"title,omitempty"`
	Version int           `json:"version,omitempty"`
	Fields  []SecretField `json:"fields,omitempty"`
}

type SecretField struct {
	Id    string `json:"id,omitempty"`
	Value string `json:"value,omitempty"`
	Label string `json:"label,omitempty"`
}

func (s Secret) ConfigSecret() ConfigSecret {
	var configSecret ConfigSecret
	for _, field := range s.Fields {
		switch slugify(field.Label) {
		case "server":
			configSecret.Server = field.Value
		case "insecure_skip_tls_verify", "insecure skip tls verify":
			configSecret.InsecureSkipTLSVerify = toBool(field.Value)
		case "certificate-authority-data", "certificate_authority", "certificate-authority", "ca":
			configSecret.CertificateAuthority = field.Value
		case "client-certificate", "client-certificate-data", "client_certificate", "cert":
			configSecret.ClientCertificate = field.Value
		case "client-key", "client-key-data", "client_key", "key":
			configSecret.ClientKey = field.Value
		}
	}
	return configSecret
}

type ExecCredential struct {
	APIVersion string            `json:"apiVersion"`
	Kind       string            `json:"kind"`
	Status     ClientCertificate `json:"status"`
}

type ClientCertificate struct {
	ClientCertificateData string `json:"clientCertificateData"`
	ClientKeyData         string `json:"clientKeyData"`
}

func main() {
	authCmd := flag.NewFlagSet("auth", flag.ExitOnError)
	authCmd.Usage = printAuthHelp

	updateCmd := flag.NewFlagSet("update", flag.ExitOnError)
	updateCmd.Usage = printUpdateHelp
	secretTag := updateCmd.String("tag", "kubeconfig", "1password tag to search for")

	if len(os.Args) < 2 {
		log.Fatal("Please provide a subcommand: 'auth [secret-name]' or 'update'")
	}

	subcommand := os.Args[1]
	// Check for help flags before proceeding
	if subcommand == "-h" || subcommand == "-help" {
		printHelp()
		return
	}

	switch subcommand {
	case "auth":
		authCmd.Parse(os.Args[2:])

		secretName := os.Args[2]
		authCommand(secretName)
	case "update":
		// add flag to control the secret tag
		updateCmd.Parse(os.Args[2:])

		updateKubeConfig(*secretTag)
	default:
		log.Fatal("Invalid subcommand. Please use 'auth' or 'update'")
	}
}

func printHelp() {
	fmt.Println(`Usage:
1password-kubeconfig [command]

Available Commands:
auth        Use this command with a secret-name to authenticate.
update      Updates the kubeconfig file based on secrets from 1password.

Flags:
-h, -help   Show help for the 1password-kubeconfig command.

Use "1password-kubeconfig [command] -h" for more information about a command.`)
}

func printAuthHelp() {
	fmt.Println(`Usage:
1password-kubeconfig auth [secret-name]

Use this command with a secret-name to authenticate.`)
}

func printUpdateHelp() {
	fmt.Println(`Usage:
1password-kubeconfig update

Flags:
-tag string  1password tag to search for

Updates the kubeconfig file based on secrets from 1password.`)
}

func updateKubeConfig(secretTag string) {
	// Example: Fetching secrets - this part would be replaced with your actual method to get secrets
	secrets := findSecretsWithTag(secretTag)

	for _, secret := range secrets {
		// Set or update cluster
		setCluster(secret)

		// Set or update credentials
		setCredentials(secret)

		// Set or update context
		setContext(secret)
	}

	fmt.Println("kubeconfig updated successfully")
}

func setCluster(secret ConfigSecret) {
	// Example command: kubectl config set-cluster NAME --server=server
	cmd := exec.Command("kubectl", "config", "set-cluster", secret.ConfigName(), "--server="+secret.Server)
	if secret.CertificateAuthority != "" {
		cmd.Args = append(cmd.Args, "--certificate-authority="+secret.CertificateAuthority)
	}
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to set cluster: %v", err)
	}
}

func setContext(secret ConfigSecret) {
	// Example command: kubectl config set-context NAME --cluster=cluster_nickname --user=user_nickname
	cmd := exec.Command("kubectl", "config", "set-context", secret.ConfigName(), "--cluster="+secret.ConfigName(), "--user="+secret.ConfigName())
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to set context: %v", err)
	}
}

func setCredentials(secret ConfigSecret) {
	cmd := exec.Command(
		"kubectl", "config",
		"set-credentials", secret.ConfigName(),
		"--exec-command=1password-kubeconfig",
		"--exec-api-version=client.authentication.k8s.io/v1",
		"--exec-arg=auth",
		"--exec-arg="+secret.Id,
	)

	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to set credentials with exec-command: %v", err)
	}
}

func authCommand(secretName string) {
	// Retrieve the secret from 1password
	secret := getSecretByName(secretName)

	execCred := ExecCredential{
		APIVersion: "client.authentication.k8s.io/v1",
		Kind:       "ExecCredential",
		Status: ClientCertificate{
			ClientCertificateData: secret.ClientCertificate,
			ClientKeyData:         secret.ClientKey,
		},
	}

	// Serialize the ExecCredential struct to JSON
	jsonOutput, err := json.Marshal(execCred)
	if err != nil {
		log.Fatalf("Failed to marshal ExecCredential to JSON: %v", err)
	}

	// Print the JSON to stdout
	fmt.Println(string(jsonOutput))
}

func findSecretsWithTag(tag string) []ConfigSecret {
	// Execute the `op` CLI command to find secrets with the specified tag
	cmd := exec.Command("op", "list", "items", "--tags", tag)
	output, err := cmd.Output()
	if err != nil {
		log.Fatalf("Failed to execute 'op' command: %v", err)
	}

	// Parse the JSON output
	var secrets []Secret
	err = json.Unmarshal(output, &secrets)
	if err != nil {
		log.Fatalf("Failed to parse 'op' command output: %v", err)
	}

	// Convert the Secret structs to ConfigSecret structs
	var configSecrets []ConfigSecret
	for _, secret := range secrets {
		configSecrets = append(configSecrets, secret.ConfigSecret())
	}

	return configSecrets
}

func getSecretByName(name string) ConfigSecret {
	// Execute the `op` CLI command to retrieve the secret by name or id
	cmd := exec.Command("op", "item", "get", name, "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		log.Fatalf("Failed to execute 'op' command: %v", err)
	}

	// Parse the JSON output
	var secret Secret
	err = json.Unmarshal(output, &secret)
	if err != nil {
		log.Fatalf("Failed to parse 'op' command output: %v", err)
	}

	return secret.ConfigSecret()
}

func toBool(s string) bool {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	switch s[0] {
	case 't', 'y', '1':
		return true
	default:
		return false
	}
}

func slugify(s string) string {
	return strings.ReplaceAll(strings.ToLower(s), " ", "-")
}
