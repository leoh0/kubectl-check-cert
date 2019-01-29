package main

import (
	"bytes"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/spf13/cast"
	yaml "gopkg.in/yaml.v2"

	"k8s.io/client-go/tools/clientcmd"

	// ref:https://github.com/kubernetes/client-go/issues/242
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func main() {
	var (
		result string
		err    error
	)

	hostName := os.Getenv("NODENAME")
	if hostName == "" {
		ReadorDie("/etc/hostname")
	}

	result, err = Execute([]string{"sh", "-c", "grep -rw kubelet /tmp/proc/*/comm | cut -d'/' -f4"})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	commands := GetCommandsFromCmdline(
		ReadorDie(fmt.Sprintf("/tmp/proc/%s/cmdline", result)))

	var (
		tempCertPath = ""
		tempKeyPath  = ""
		// A and B 여야함
		isServerRotateCertA = false
		isServerRotateCertB = false

		isClientRotateCert    = false
		isServerRotateCert    = false
		kubeletServerCertPath = path.Join(defaultKubeletServerCertPath, "kubelet.crt")
	)

	configData := map[string]interface{}{}
	if configPath, ok := commands[configFlag]; ok {
		err = yaml.Unmarshal(
			[]byte(ReadorDie(configPath)), &configData)

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	if data, ok := commands[tlsCertFlag]; ok {
		tempCertPath = data
	} else if data, ok := configData[tlsCertOption]; ok {
		tempCertPath = cast.ToString(data)
	}

	if data, ok := commands[tlsKeyFlag]; ok {
		tempKeyPath = data
	} else if data, ok := configData[tlsKeyOption]; ok {
		tempKeyPath = cast.ToString(data)
	}

	if data, ok := commands[rotateServerCertFlag]; ok {
		isServerRotateCertA = cast.ToBool(data)
	} else if data, ok := configData[rotateServerCertOption]; ok {
		isServerRotateCertA = cast.ToBool(data)
	}

	for _, v := range strings.Split(commands[featureGatesFlag], ",") {
		value := strings.Split(v, "=")
		if value[0] == rotateKubeletServerCertFeature {
			isServerRotateCertB = cast.ToBool(value[1])
		}
	}
	if features, ok := configData[featureGatesOption].(map[interface{}]interface{}); ok {
		if data, ok := features[rotateKubeletServerCertFeature]; ok {
			isServerRotateCertB = cast.ToBool(data)
		}
	}

	isServerRotateCert = isServerRotateCertA && isServerRotateCertB

	if tempCertPath != "" && tempKeyPath != "" {
		kubeletServerCertPath = tempCertPath
	} else if isServerRotateCert {
		kubeletServerCertPath = path.Join(defaultKubeletServerCertPath, "kubelet-server-current.pem")
	} else if data, ok := commands[certDirFlag]; ok {
		kubeletServerCertPath = path.Join(data, "kubelet.crt")
	}

	date, days := GetDateAndDaysFromCert(
		ReadorDie(kubeletServerCertPath))

	e := []Entry{}
	e = append(e, *NewEntry(hostName, "server-cert", days, date, kubeletServerCertPath))

	if data, ok := commands[rotateCertFlag]; ok {
		isClientRotateCert = cast.ToBool(data)
	} else if data, ok := configData[rotateCertOption]; ok {
		isClientRotateCert = cast.ToBool(data)
	}

	// TODO: also return this value
	_ = isClientRotateCert

	if kubeConfigPath, ok := commands[kubeConfigFlag]; ok {
		cfg, err := clientcmd.NewClientConfigFromBytes(
			[]byte(ReadorDie(kubeConfigPath)))
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		rawConfig, err := cfg.RawConfig()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		var (
			kubeletClientCertFile string
			kubeletClientCertPath string
		)
		currentContext := rawConfig.CurrentContext
		user := rawConfig.Contexts[currentContext].AuthInfo
		u := rawConfig.AuthInfos[user]

		if string(u.ClientCertificateData) != "" {
			kubeletClientCertFile = string(u.ClientCertificateData)
			kubeletClientCertPath = kubeConfigPath
		} else if string(u.ClientCertificate) != "" {
			kubeletClientCertFile = ReadorDie(strings.Trim(string(u.ClientCertificate), "\n"))
			kubeletClientCertPath = string(u.ClientCertificate)
		} else {
			fmt.Println(err)
			os.Exit(1)
		}

		date, days := GetDateAndDaysFromCert(kubeletClientCertFile)

		e = append(e, *NewEntry(hostName, "client-cert", days, date, kubeletClientCertPath))

	} else {
		fmt.Println(err)
		os.Exit(1)
	}

	o := Output{
		Entries: e,
	}

	buffer := &bytes.Buffer{}
	if err := json.NewEncoder(buffer).Encode(o); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// print to stdout
	fmt.Print(buffer.String())
}

// Execute run command and return it's output
func Execute(cmd []string) (string, error) {
	var (
		out []byte
		err error
	)
	if len(cmd) > 1 {
		out, err = exec.Command(cmd[0], cmd[1:]...).Output()
	} else {
		out, err = exec.Command(cmd[0]).Output()
	}
	if err != nil {
		fmt.Printf("Failed to execute command: %s\n", strings.Join(cmd, " "))
		return "", err
	}

	return strings.Trim(string(out), "\n"), nil
}

// GetDateAndDaysFromCert takes a cert and extract Date and Days
func GetDateAndDaysFromCert(cert string) (time.Time, int) {
	timeNow := time.Now()
	block, _ := pem.Decode([]byte(cert))
	if block == nil {
		fmt.Println("failed to parse certificate PEM")
		os.Exit(1)
	}
	c, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		fmt.Println("failed to parse certificate: " + err.Error())
		os.Exit(1)
	}
	expiresIn := int(c.NotAfter.Sub(timeNow).Hours() / 24)

	return c.NotAfter, expiresIn
}

// GetCommandsFromCmdline takes a cmdline and parse it to several commands
func GetCommandsFromCmdline(cmdline string) map[string]string {
	var (
		cmds  = map[string]string{}
		value string
	)

	for _, c := range strings.Split(cmdline, "--") {
		c = string(bytes.Trim([]byte(c), "\x00"))
		s := strings.Split(c, "=")

		if len(s) == 1 {
			value = "true"
		} else {
			value = strings.Join(s[1:], "=")
		}

		cmds[s[0]] = strings.Trim(value, "\n")
	}

	return cmds
}

// ReadorDie read fileName file and return it's content
func ReadorDie(fileName string) string {
	body, err := ioutil.ReadFile(fileName)
	if err != nil {
		fmt.Println("Read file fail: " + fileName + err.Error())
		os.Exit(1)
	}

	return strings.Trim(string(body), "\n")
}
