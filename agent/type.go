package main

import "time"

const (
	// only in flag
	configFlag  = "config"
	certDirFlag = "cert-dir"

	// flag and options
	kubeConfigFlag       = "kubeconfig"
	rotateCertFlag       = "rotate-certificates"
	rotateServerCertFlag = "rotate-server-certificates"
	tlsCertFlag          = "tls-cert-file"
	tlsKeyFlag           = "tls-key-file"
	featureGatesFlag     = "feature-gates"

	kubeConfigOption       = "kubeconfig"
	rotateCertOption       = "rotateCertificates"
	rotateServerCertOption = "serverTLSBootstrap"
	tlsCertOption          = "tlsCertFile"
	tlsKeyOption           = "tlsKeyFile"
	featureGatesOption     = "featureGates"

	rotateKubeletServerCertFeature = "RotateKubeletServerCertificate"

	defaultKubeletServerCertPath = "/var/lib/kubelet/pki/"

	entryType = "kubelet"
)

// Output is for json stdout
type Output struct {
	Entries []Entry `json:"entry"`
}

// Entry is for cert entries
type Entry struct {
	Type string    `json:"type"`
	Node string    `json:"node"`
	Name string    `json:"name"`
	Days int       `json:"days"`
	Due  time.Time `json:"due"`
	Path string    `json:"path"`
}

// NewEntry make entry
func NewEntry(node string, name string, days int, due time.Time, path string) *Entry {
	return &Entry{
		Type: entryType,
		Node: node,
		Name: name,
		Days: days,
		Due:  due,
		Path: path,
	}
}
