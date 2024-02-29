package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/miekg/dns"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

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
				records = append(records, arpa)
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

func CreateUpdateDnsMasqConfig(ctx context.Context, client client.Client, header string, records []string) error {
	logr := log.FromContext(ctx)
	header = strings.ReplaceAll(header, "port=53", "port=25353")
	hosts := ToHosts(records)
	completeConfig := fmt.Sprintf("%s\n%s", header, hosts)

	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dnsmasq-config",
			Namespace: "vsphere-infra-helpers",
		},
	}

	err := client.Get(ctx, types.NamespacedName{Name: "dnsmasq-config", Namespace: "vsphere-infra-helpers"}, &cm)
	if err != nil {
		logr.Info("dnsmasq-config configmap not found, will create")
		cm.Data = map[string]string{
			"dnsmasq.cfg": completeConfig,
		}
		err = client.Create(ctx, &cm)
		if err != nil {
			logr.Error(err, "unable to create dnsmasq-config configmap")
		}
	} else {
		cm.Data = map[string]string{
			"dnsmasq.cfg": completeConfig,
		}
		err := client.Update(ctx, &cm)
		if err != nil {
			logr.Error(err, "unable to update dnsmasq-config configmap")
		}
	}

	return nil
}
