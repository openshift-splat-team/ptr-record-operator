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
		controller.StartManager(additionalCIDR)
	},
}

func init() {
	rootCmd.AddCommand(monitorCmd)
	monitorCmd.PersistentFlags().StringVar(&additionalCIDR, "cidr", "", "additional CIDR for which to generate reverse DNS records")
}
