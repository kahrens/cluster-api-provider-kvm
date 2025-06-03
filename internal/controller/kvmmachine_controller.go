/*
Copyright 2025.

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
	"os/exec"
	"time"

	"github.com/libvirt/libvirt-go"
	libvirtxml "github.com/libvirt/libvirt-go-xml"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kvmv1alpha1 "github.com/kahrens/cluster-api-provider-kvm/api/v1alpha1"
	clusterapiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// KVMMachineReconciler reconciles a KVMMachine object
type KVMMachineReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kvm.cluster.x-k8s.io,resources=kvmmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kvm.cluster.x-k8s.io,resources=kvmmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kvm.cluster.x-k8s.io,resources=kvmmachines/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the KVMMachine object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *KVMMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = logf.FromContext(ctx) // NEW

	logger := log.FromContext(ctx) // OLD

	// Fetch the KVMVirtualMachine
	vm := &kvmv1alpha1.KVMVirtualMachine{}
	if err := r.Get(ctx, req.NamespacedName, vm); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get KVMVirtualMachine")
		return ctrl.Result{}, err
	}

	// Connect to libvirt
	conn, err := libvirt.NewConnect("qemu+:///" + vm.Spec.Host + "/system")
	if err != nil {
		logger.Error(err, "Failed to connect to libvirt")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}
	defer conn.Close()

	// Check if VM exists
	dom, err := conn.LookupDomainByName(vm.Spec.VMName)
	if err != nil && err.(libvirt.Error).Code != libvirt.ERR_NO_DOMAIN {
		logger.Error(err, "Failed to lookup VM")
		return ctrl.Result{}, err
	}

	if dom == nil {
		// Create VM
		logger.Info("Creating VM", "name", vm.Spec.VMName)
		domCfg := &libvirtxml.Domain{
			Type: "kvm",
			Name: vm.Spec.VMName,
			Memory: &libvirtxml.DomainMemory{
				Value: 8 * 1024 * 1024, // 8Gi in KiB
				Unit:  "KiB",
			},
			VCPU: &libvirtxml.DomainVCPU{
				Placement: "static",
				Value:     vm.Spec.CPUs,
			},
			OS: &libvirtxml.DomainOS{
				Type: &libvirtxml.DomainOSType{
					Type: "hvm",
				},
			},
			Devices: &libvirtxml.DomainDeviceList{
				Disks: []libvirtxml.DomainDisk{
					{
						Device: "disk",
						Driver: &libvirtxml.DomainDiskDriver{
							Name: "qemu",
							Type: "qcow2",
						},
						Source: &libvirtxml.DomainDiskSource{
							File: &libvirtxml.DomainDiskSourceFile{
								File: "/tmp/" + vm.Spec.VMName + ".qcow2",
							},
						},
						Target: &libvirtxml.DomainDiskTarget{
							Dev: "vda",
							Bus: "virtio",
						},
					},
					{
						Device: "cdrom",
						Source: &libvirtxml.DomainDiskSource{
							File: &libvirtxml.DomainDiskSourceFile{
								File: "/tmp/" + vm.Spec.VMName + "-cloudinit.iso",
							},
						},
						Target: &libvirtxml.DomainDiskTarget{
							Dev: "hdc",
							Bus: "ide",
						},
					},
				},
				Interfaces: []libvirtxml.DomainInterface{
					{
						Model: &libvirtxml.DomainInterfaceModel{
							Type: "virtio",
						},
						Source: &libvirtxml.DomainInterfaceSource{
							Bridge: &libvirtxml.DomainInterfaceSourceBridge{
								Bridge: vm.Spec.NetworkInterface.Bridge,
							},
						},
					},
				},
			},
		}

		// Create disk image
		_, err = execCommand("qemu-img", "create", "-f", "qcow2", "-b", vm.Spec.ImagePath, "/tmp/"+vm.Spec.VMName+".qcow2")
		if err != nil {
			logger.Error(err, "Failed to create disk image")
			return ctrl.Result{}, err
		}

		// Create cloud-init ISO
		_, err = execCommand("genisoimage", "-output", "/tmp/"+vm.Spec.VMName+"-cloudinit.iso", "-volid", "cidata", "-joliet", "-rock-data", "user-data="+vm.Spec.CloudInit, "meta-data=hostname:"+vm.Spec.VMName)
		if err != nil {
			logger.Error(err, "Failed to create cloud-init ISO")
			return ctrl.Result{}, err
		}

		// Define and start VM
		xml, err := domCfg.Marshal()
		if err != nil {
			return ctrl.Result{}, err
		}
		dom, err = conn.DomainDefineXML(xml)
		if err != nil {
			logger.Error(err, "Failed to define VM")
			return ctrl.Result{}, err
		}
		err = dom.Create()
		if err != nil {
			logger.Error(err, "Failed to start VM")
			return ctrl.Result{}, err
		}
	}

	// Update status
	vm.Status.Ready = true
	// TODO: Fetch IP address from VM (e.g., via libvirt or DHCP lease)
	vm.Status.IPAddress = "192.168.1.100" // Placeholder
	if err := r.Status().Update(ctx, vm); err != nil {
		logger.Error(err, "Failed to update KVMVirtualMachine status")
		return ctrl.Result{}, err
	}

	// Update corresponding Machine status
	machine := &clusterapiv1beta1.Machine{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: vm.Namespace, Name: vm.Name}, machine); err == nil {
		machine.Status.Addresses = []clusterapiv1beta1.MachineAddress{
			{
				Type:    clusterapiv1beta1.MachineExternalIP,
				Address: vm.Status.IPAddress,
			},
		}
		if err := r.Status().Update(ctx, machine); err != nil {
			logger.Error(err, "Failed to update Machine status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KVMMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kvmv1alpha1.KVMMachine{}).
		Named("kvmmachine").
		Complete(r)
}

// execCommand is a helper to run shell commands
func execCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}
