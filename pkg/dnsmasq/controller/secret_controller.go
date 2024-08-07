/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/miekg/dns"
	vcmv1 "github.com/openshift-splat-team/vsphere-capacity-manager/pkg/apis/vspherecapacitymanager.splat.io/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

// SecretReconciler reconciles a HaproxyMetal object
type SecretReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	AdditionalCIDR string
	PrivateKeyPath string
	DnsServer      string
}

// incIP increments an IP address.
func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func processCIDR(ctx context.Context, cidr string) ([]string, error) {
	logr := log.FromContext(ctx)
	logr.V(1).Info("processing CIDR", "cidr", cidr)
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	var ptrRecords []string
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); incIP(ip) {
		addr, err := dns.ReverseAddr(ip.String())
		if err != nil {
			return nil, fmt.Errorf("unable to reverse address: %v", err)
		}
		ptrRecords = append(ptrRecords, fmt.Sprintf("%s %s", ip.String(), addr))
	}
	return ptrRecords, nil
}

// +kubebuilder:rbac:groups=v1,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
func (r *SecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logr := log.FromContext(ctx)
	logr.V(1).Info("reconciling Secret")
	secret := &corev1.Secret{}

	var records []string
	err := r.Client.Get(ctx, req.NamespacedName, secret)
	if err != nil {
		logr.Error(err, "unable to fetch secret")
		return ctrl.Result{}, err
	}
	if val, exists := secret.Data["subnets.json"]; exists {
		records, err := SubnetParse(string(val))
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to parse subnets.json: %v", err)
		}

		if r.AdditionalCIDR != "" {
			additionalRecords, err := processCIDR(ctx, r.AdditionalCIDR)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("unable to process additional CIDR: %v", err)
			}
			records = append(records, additionalRecords...)
		}
	}

	var networkList vcmv1.NetworkList

	err = r.Client.List(ctx, &networkList, client.InNamespace("vsphere-infra-helpers"))
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to list networks: %v", err)
	}
	for _, network := range networkList.Items {
		logr.V(1).Info("processing VCM network", "network", network.Name)
		additionalRecords, err := processCIDR(ctx, network.Spec.MachineNetworkCidr)
		if err != nil {
			logr.V(1).Info(fmt.Sprintf("unable to process additional CIDR: %v", err))
			continue
		}
		logr.V(1).Info(fmt.Sprintf("appending %d records", len(additionalRecords)))
		records = append(records, additionalRecords...)
	}

	err = UpdateDNSHost(ctx, r.Client, r.PrivateKeyPath, r.DnsServer, string(secret.Data["dnsmasq.cfg"]), records)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to update ci-dns.OCP-vsphere.cloud with additional hosts: %v", err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}, builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}, predicate.NewPredicateFuncs(func(object client.Object) bool {
			return object.GetNamespace() == "test-credentials" && object.GetName() == "vsphere-config"
		}))).
		Complete(r)
}

func StartManager(context SecretReconciler) {
	var metricsAddr string
	var namespace string
	var enableLeaderElection bool
	var probeAddr string

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&namespace, "namespace", "vsphere-infra-helpers", "The namespace where ")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "0210028e.vanderlab.net",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	client := mgr.GetClient()
	corev1.AddToScheme(mgr.GetScheme())
	appsv1.AddToScheme(mgr.GetScheme())
	vcmv1.AddToScheme(mgr.GetScheme())
	if err = (&SecretReconciler{
		Client:         client,
		Scheme:         mgr.GetScheme(),
		AdditionalCIDR: context.AdditionalCIDR,
		PrivateKeyPath: context.PrivateKeyPath,
		DnsServer:      context.DnsServer,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "namespace")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}

	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}

}
