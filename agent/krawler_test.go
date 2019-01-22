package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cast"

	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v2"
)

func TestParsingYaml(t *testing.T) {
	kubeletConfigFile := `
address: 0.0.0.0
apiVersion: kubelet.config.k8s.io/v1beta1
authentication:
  anonymous:
    enabled: false
  webhook:
    cacheTTL: 2m0s
    enabled: true
  x509:
    clientCAFile: /etc/kubernetes/pki/ca.crt
authorization:
  mode: Webhook
  webhook:
    cacheAuthorizedTTL: 5m0s
    cacheUnauthorizedTTL: 30s
cgroupDriver: cgroupfs
cgroupsPerQOS: true
clusterDNS:
- 10.96.0.10
clusterDomain: cluster.local
containerLogMaxFiles: 5
containerLogMaxSize: 10Mi
contentType: application/vnd.kubernetes.protobuf
cpuCFSQuota: true
cpuManagerPolicy: none
cpuManagerReconcilePeriod: 10s
enableControllerAttachDetach: true
enableDebuggingHandlers: true
enforceNodeAllocatable:
- pods
eventBurst: 10
eventRecordQPS: 5
evictionHard:
  imagefs.available: 15%
  memory.available: 100Mi
  nodefs.available: 10%
  nodefs.inodesFree: 5%
evictionPressureTransitionPeriod: 5m0s
failSwapOn: true
fileCheckFrequency: 20s
hairpinMode: promiscuous-bridge
healthzBindAddress: 127.0.0.1
healthzPort: 10248
httpCheckFrequency: 20s
imageGCHighThresholdPercent: 85
imageGCLowThresholdPercent: 80
imageMinimumGCAge: 2m0s
iptablesDropBit: 15
iptablesMasqueradeBit: 14
kind: KubeletConfiguration
kubeAPIBurst: 10
kubeAPIQPS: 5
makeIPTablesUtilChains: true
maxOpenFiles: 1000000
maxPods: 110
nodeStatusUpdateFrequency: 10s
oomScoreAdj: -999
podPidsLimit: -1
port: 10250
registryBurst: 10
registryPullQPS: 5
resolvConf: /etc/resolv.conf
rotateCertificates: true
runtimeRequestTimeout: 2m0s
serializeImagePulls: true
staticPodPath: /etc/kubernetes/manifests
streamingConnectionIdleTimeout: 4h0m0s
syncFrequency: 1m0s
volumeStatsAggPeriod: 1m0s
featureGates:
  RotateKubeletServerCertificate: true
`

	config := map[string]interface{}{}
	_ = yaml.Unmarshal([]byte(kubeletConfigFile), &config)

	assert.Equal(t, true, config[rotateCertOption])

	if features, ok := config[featureGatesOption].(map[interface{}]interface{}); ok {
		if data, ok := features[rotateKubeletServerCertFeature]; ok {
			assert.Equal(t, true, data)
		}
	}

}

func TestParsingFlag(t *testing.T) {
	s := "/usr/bin/kubelet--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf--kubeconfig=/etc/kubernetes/kubelet.conf--config=/var/lib/kubelet/config.yaml--cgroup-driver=systemd--cni-bin-dir=/opt/cni/bin--cni-conf-dir=/etc/cni/net.d--network-plugin=cni--feature-gates=RotateKubeletServerCertificate=true--allowed-unsafe-sysctls=net.*"
	commands := GetCommandsFromCmdline(s)

	assert.Equal(t, "/var/lib/kubelet/config.yaml", commands[configFlag])
	assert.Equal(t, "", commands[certDirFlag])
	assert.Equal(t, "/etc/kubernetes/kubelet.conf", commands[kubeConfigFlag])
	assert.Equal(t, "", commands[rotateCertFlag])
	assert.Equal(t, "", commands[rotateServerCertFlag])
	assert.Equal(t, "", commands[tlsCertFlag])
	assert.Equal(t, "", commands[tlsKeyFlag])
	assert.Equal(t, "RotateKubeletServerCertificate=true", commands[featureGatesFlag])
	for _, v := range strings.Split(commands[featureGatesFlag], ",") {
		value := strings.Split(v, "=")
		if value[0] == rotateKubeletServerCertFeature {
			assert.Equal(t, true, cast.ToBool(value[1]))
		}
	}
}

func TestJsonEncode(t *testing.T) {
	o := Output{}
	o.Entries = append(o.Entries, *NewEntry("node", "name", 1, time.Date(2019, time.January, 1, 0, 0, 0, 0, time.UTC), "path"))

	buffer := &bytes.Buffer{}
	if err := json.NewEncoder(buffer).Encode(o); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	t.Log(buffer.String())
	assert.Equal(t, "{\"entry\":[{\"type\":\"kubelet\",\"node\":\"node\",\"name\":\"name\",\"days\":1,\"due\":\"2019-01-01T00:00:00Z\",\"path\":\"path\"}]}\n", buffer.String())
}
