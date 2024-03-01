/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/openshift-splat-team/vsphere-ci-dns/pkg/dnsmasq/controller"
	"github.com/spf13/cobra"
)

var (
	additionalCIDR string
	privateKeyPath string
	dnsServer	  string
)

// monitorCmd represents the monitor command
var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		controller.StartManager(controller.SecretReconciler{
			AdditionalCIDR: additionalCIDR,
			PrivateKeyPath: privateKeyPath,
			DnsServer: dnsServer,
		})
	},
}

func init() {
	rootCmd.AddCommand(monitorCmd)
	monitorCmd.PersistentFlags().StringVar(&additionalCIDR, "cidr", "192.168.0.0/16", "additional CIDR for which to generate reverse DNS records")
	monitorCmd.PersistentFlags().StringVar(&privateKeyPath, "private-key", "/ssh-config/private-key", "path to a private key for SSH access to the DNS server")
	monitorCmd.PersistentFlags().StringVar(&dnsServer, "dns-server", "10.176.158.144", "additional CIDR for which to generate reverse DNS records")
}
