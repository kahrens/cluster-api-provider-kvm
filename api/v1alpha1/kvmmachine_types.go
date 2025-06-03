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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// KVMMachineSpec defines the desired state of KVMMachine.
type KVMMachineSpec struct {
	// VMName is the name of the VM in libvirt
	VMName string `json:"vmName"`
	// Host is the blade server hostname or IP running the VM
	Host string `json:"host"`
	// CPUs is the number of CPU cores
	CPUs int `json:"cpus"`
	// Memory is the memory size (e.g., "8Gi")
	Memory string `json:"memory"`
	// DiskSize is the disk size (e.g., "20Gi")
	DiskSize string `json:"diskSize"`
	// ImagePath is the path to the base qcow2 image
	ImagePath string `json:"imagePath"`
	// CloudInit is the cloud-init user data
	CloudInit string `json:"cloudInit"`
	// NetworkInterface is the network configuration
	NetworkInterface NetworkInterface `json:"networkInterface"`
}

// NetworkInterface defines the network configuration
type NetworkInterface struct {
	Bridge string `json:"bridge"`
}

// KVMMachineStatus defines the observed state of KVMMachine.
type KVMMachineStatus struct {
	// Ready indicates if the VM is running and accessible
	Ready bool `json:"ready"`
	// IPAddress is the VM’s IP address
	IPAddress string `json:"ipAddress,omitempty"`
	// Conditions store the status conditions of the VM
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// KVMMachine is the Schema for the kvmmachines API.
type KVMMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KVMMachineSpec   `json:"spec,omitempty"`
	Status KVMMachineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KVMMachineList contains a list of KVMMachine.
type KVMMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KVMMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KVMMachine{}, &KVMMachineList{})
}
