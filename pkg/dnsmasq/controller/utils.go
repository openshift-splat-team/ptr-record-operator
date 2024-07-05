package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	scp "github.com/bramvdbogaerde/go-scp"
	"github.com/miekg/dns"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func provisionHosts(ctx context.Context, client client.Client, privateKeyPath, server, hosts string) error {
	logr := log.FromContext(ctx)

	logr.Info("provisioning hosts file", "server", server)

	// Load your private key
	key, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return errors.Wrapf(err, "unable to read private key")
	}

	// Create the Signer for this private key
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return errors.Wrapf(err, "unable to parse private key")
	}

	logr.Info("parsed private key")
	config := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         0,
	}

	sshHost := fmt.Sprintf("%s:22", server)

	logr.Info("connecting to server")
	sshClient, err := ssh.Dial("tcp", sshHost, config)
	if err != nil {
		return errors.Wrapf(err, "unable to dial")
	}

	logr.Info("connected to server")

	// Create a new SCP client
	scpClient := scp.NewClient(sshHost, config)

	// Connect to the remote server
	err = scpClient.Connect()
	if err != nil {
		return errors.Wrapf(err, "unable to connect")
	}
	defer scpClient.Close()

	logr.Info("copying hosts file")
	err = scpClient.Copy(ctx, strings.NewReader(hosts), "/opt/ci-dns/additional-hosts", "0666", int64(len(hosts)))
	if err != nil {
		return errors.Wrapf(err, "unable to copy")
	}

	// Create a session
	session, err := sshClient.NewSession()
	if err != nil {
		return errors.Wrapf(err, "unable to create session")
	}
	defer session.Close()

	logr.Info("restart dnsmasq")
	err = session.Run("sudo systemctl restart dnsmasq")
	if err != nil {
		return errors.Wrapf(err, "unable to restart dnsmasq")
	}

	logr.Info("restarted dnsmasq")

	return nil
}

// SubnetParse parses a json file and returns a list of reverse DNS records
func SubnetParse(content string) ([]string, error) {
	records := []string{}
	var subnetsUntyped map[string]interface{}
	if err := json.Unmarshal([]byte(content), &subnetsUntyped); err != nil {
		return nil, errors.Wrapf(err, "unable to parse")
	}

	for _, vlans := range subnetsUntyped {
		for _, subnetUntyped := range vlans.(map[string]interface{}) {
			subnet := subnetUntyped.(map[string]interface{})
			ipAddresses := subnet["ipAddresses"].([]interface{})
			for _, ip := range ipAddresses {
				arpa, err := dns.ReverseAddr(ip.(string))
				if err != nil {
					return nil, errors.Wrapf(err, "unable to reverse address")
				}
				records = append(records, fmt.Sprintf("%s %s", ip.(string), arpa))
			}
		}
	}
	return records, nil
}

// ToHosts converts a list of records to a hosts file format
func ToHosts(records []string) string {
	var builder strings.Builder

	for _, record := range records {
		builder.WriteString(fmt.Sprintf("ptr-record=%s\n", record))
	}
	return builder.String()
}

func UpdateDNSHost(ctx context.Context, client client.Client, privateKeyPath, server, header string, records []string) error {
	logr := log.FromContext(ctx)
	logr.Info("updating DNS host")
	logr.V(1).Info("records count", "records", len(records))
	allKeys := make(map[string]bool)
	list := []string{}
	for _, item := range records {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			list = append(list, item)
		}
	}
	logr.V(1).Info("records after duplicate removal", "records", len(list))
	return provisionHosts(ctx, client, privateKeyPath, server, strings.Join(list, "\n"))
}
