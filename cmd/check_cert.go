package cmd

import (
	"bytes"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/common/log"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"
	pb "gopkg.in/cheggaaa/pb.v1"

	"k8s.io/client-go/kubernetes/scheme"
	appsV1Client "k8s.io/client-go/kubernetes/typed/apps/v1"
	coreV1Client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"

	// ref:https://github.com/kubernetes/client-go/issues/242
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/olekukonko/tablewriter"
)

const (
	kubesystemNamespace = "kube-system"
	defaultNamespace    = "default"

	kubeletCAFlag = "kubelet-certificate-authority"

	name              = "krawler"
	imageName         = "leoh0/krawler"
	etcKubernetesPath = "/etc/kubernetes/"
	etcKubernetesName = "etc-kubernetes"
	varLibKubeletPath = "/var/lib/kubelet/"
	varLibKubeletName = "var-lib-kubelet"
	hostnamePath      = "/etc/hostname"
	hostnameName      = "hostname"
	realProcPath      = "/proc/"
	tmpProcPath       = "/tmp/proc/"
	tmpProcName       = "tmp-proc"
)

var (
	expirationExample = `
	# view expiration days of certifications about control plane (e.g. apiserver, controller-manager, scheduler)
	%[1]s check-cert

	# view expiration days of certifications about control plane and also kubelets by installing crawling daemon-set
	%[1]s check-cert --also-check-kubelet
`

	certOptions = []string{"etcd-certfile", "tls-cert-file", "kubelet-client-certificate", "proxy-client-cert-file"}

	matchLabel = map[string]string{"app": name}
	max        = intstr.FromString("100%")

	hostPathType     = corev1.HostPathDirectory
	hostPathFileType = corev1.HostPathFile

	checkKubeletWithCA = false
	clientConfig       *rest.Config
)

type serverCertification struct {
	Entry   Entry
	Warning string
}

// ExpirationOptions provides information
type ExpirationOptions struct {
	configFlags *genericclioptions.ConfigFlags
	genericclioptions.IOStreams

	checkKubelet bool
}

// NewExpirationOptions provides an instance of ExpirationOptions with default values
func NewExpirationOptions(streams genericclioptions.IOStreams) *ExpirationOptions {
	return &ExpirationOptions{
		configFlags:  genericclioptions.NewConfigFlags(true),
		checkKubelet: false,
		IOStreams:    streams,
	}
}

// NewCmdExpiration provides a cobra command wrapping ExpirationOptions
func NewCmdExpiration(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewExpirationOptions(streams)

	cmd := &cobra.Command{
		Use:          "check-cert [flags]",
		Short:        "View expiration days of certifications in kubernetes cluster",
		Example:      fmt.Sprintf(expirationExample, "kubectl"),
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {
			if err := o.Run(c); err != nil {
				return err
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&o.checkKubelet, "also-check-kubelet", false, "if true, also check kubelet certification")
	o.configFlags.AddFlags(cmd.Flags())

	return cmd
}

func getPods(coreclient *coreV1Client.CoreV1Client, namespace string, label string) (*corev1.PodList, error) {
	listOption := meta_v1.ListOptions{
		LabelSelector: label,
	}

	pods, err := coreclient.Pods(namespace).List(listOption)
	if len(pods.Items) == 0 || err != nil {
		return &corev1.PodList{}, err
	}

	return pods, nil
}

func isJSON(s string) (*Output, bool) {
	var e Output
	val := json.Unmarshal([]byte(s), &e)
	return &e, val == nil
}

// Run gather all information
func (o *ExpirationOptions) Run(cmd *cobra.Command) error {
	var err error
	clientConfig, err = o.configFlags.ToRESTConfig()
	if err != nil {
		return err
	}
	coreclient, err := coreV1Client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	if o.checkKubelet {
		daemonSet := &appv1.DaemonSet{
			ObjectMeta: meta_v1.ObjectMeta{
				Name: name,
			},
			Spec: appv1.DaemonSetSpec{
				UpdateStrategy: appv1.DaemonSetUpdateStrategy{
					Type: appv1.RollingUpdateDaemonSetStrategyType,
					RollingUpdate: &appv1.RollingUpdateDaemonSet{
						MaxUnavailable: &max,
					},
				},
				Selector: &meta_v1.LabelSelector{
					MatchLabels: matchLabel,
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: meta_v1.ObjectMeta{
						Labels: matchLabel,
					},
					Spec: corev1.PodSpec{
						HostPID:     true,
						HostNetwork: true,
						Containers: []corev1.Container{{
							Name:            name,
							Image:           imageName,
							ImagePullPolicy: corev1.PullAlways,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      etcKubernetesName,
									MountPath: etcKubernetesPath,
								}, {
									Name:      varLibKubeletName,
									MountPath: varLibKubeletPath,
								}, {
									Name:      hostnameName,
									MountPath: hostnamePath,
								}, {
									Name:      tmpProcName,
									MountPath: tmpProcPath,
								},
							},
						}},
						RestartPolicy: corev1.RestartPolicyAlways,
						Tolerations: []corev1.Toleration{{
							Operator: corev1.TolerationOpExists,
						}},
						Volumes: []corev1.Volume{
							{
								Name: etcKubernetesName,
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Type: &hostPathType,
										Path: etcKubernetesPath,
									},
								},
							}, {
								Name: varLibKubeletName,
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Type: &hostPathType,
										Path: varLibKubeletPath,
									},
								},
							}, {
								Name: hostnameName,
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Type: &hostPathFileType,
										Path: hostnamePath,
									},
								},
							}, {
								Name: tmpProcName,
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Type: &hostPathType,
										Path: realProcPath,
									},
								},
							},
						},
					},
				},
			},
		}

		appClient, err := appsV1Client.NewForConfig(clientConfig)
		if err != nil {
			return err
		}

		_, err = appClient.DaemonSets(defaultNamespace).Create(daemonSet)

		defer appClient.DaemonSets(defaultNamespace).Delete(name, nil)
	}

	apiServerPods, err := getPods(
		coreclient, kubesystemNamespace, "component=kube-apiserver,tier=control-plane")
	if err != nil {
		fmt.Println("Apiserver is not exists. Skip.")
	}

	controllerManagerPods, err := getPods(
		coreclient, kubesystemNamespace, "component=kube-controller-manager,tier=control-plane")
	if err != nil {
		fmt.Println("ControllerManager is not exists. Skip.")
	}

	schedulerManagerPods, err := getPods(
		coreclient, kubesystemNamespace, "component=kube-scheduler,tier=control-plane")
	if err != nil {
		fmt.Println("Scheduler is not exists. Skip.")
	}

	dsPodCount := 0
	if o.checkKubelet {
		appClient, err := appsV1Client.NewForConfig(clientConfig)
		if err != nil {
			return err
		}

		dsClient := appClient.DaemonSets(defaultNamespace)
		if err != nil {
			fmt.Println("Exist already")
		} else {
			for {
				time.Sleep(time.Second / 2)

				ds, err := dsClient.Get(name, meta_v1.GetOptions{})
				if err != nil {
					return err
				}
				if ds.Status.DesiredNumberScheduled > 0 {
					dsPodCount = int(ds.Status.DesiredNumberScheduled)
					break
				}
			}
		}
	}

	bar := pb.New(len(apiServerPods.Items) + len(controllerManagerPods.Items) + len(schedulerManagerPods.Items) + dsPodCount)
	bar.SetWidth(80)
	bar.SetMaxWidth(80)
	bar.Start()

	var wg sync.WaitGroup
	var mutex = &sync.Mutex{}

	channel := make(chan interface{})

	serverCertifications := make([]serverCertification, 0)
	for _, apiServerPod := range apiServerPods.Items {
		wg.Add(1)
		go func(p corev1.Pod) {
			defer wg.Done()
			for _, c := range p.Spec.Containers[0].Command {
				// only for options
				if strings.HasPrefix(c, "--") {
					s := strings.Split(c[2:], "=")
					if s[0] == kubeletCAFlag && s[1] != "" {
						mutex.Lock()
						checkKubeletWithCA = true
						mutex.Unlock()
					}
					for _, co := range certOptions {
						if co == s[0] {
							cert, err := ExecPod(coreclient, kubesystemNamespace, &p, []string{"cat", s[1]})
							errorStr := ""
							if err != nil {
								errorStr = err.Error()
							} else {
								date, days, err := GetDateAndDaysFromCert(cert)
								if err != nil {
									errorStr = err.Error()
								} else {
									channel <- serverCertification{
										Entry: Entry{
											Type: "apiserver",
											Node: p.Spec.NodeName,
											Name: s[0],
											Path: s[1],
											Days: days,
											Due:  date,
										},
										Warning: "",
									}
								}
							}
							if errorStr != "" {
								channel <- serverCertification{
									Entry: Entry{
										Type: "apiserver",
										Node: p.Spec.NodeName,
										Name: s[0],
										Path: s[1],
										Days: 0,
										Due:  time.Time{},
									},
									Warning: err.Error(),
								}
							}
						}
					}
				}
			}
			mutex.Lock()
			bar.Increment()
			mutex.Unlock()
		}(apiServerPod)
	}

	for _, controllerManagerPod := range controllerManagerPods.Items {
		wg.Add(1)
		go func(p corev1.Pod) {
			defer wg.Done()
			for _, c := range p.Spec.Containers[0].Command {
				// only for options
				if strings.HasPrefix(c, "--") {
					s := strings.Split(c[2:], "=")
					if s[0] == kubeConfigFlag {
						errorResult := func(node string, errstr string) {
							channel <- serverCertification{
								Entry: Entry{
									Type: "controller-manager",
									Node: node,
									Name: "client-cert",
									Path: "-",
									Days: 0,
									Due:  time.Time{},
								},
								Warning: errstr,
							}
						}
						kubeconfig, err := ExecPod(coreclient, kubesystemNamespace, &p, []string{"cat", s[1]})
						if err != nil {
							errorResult(p.Spec.NodeName, err.Error())
						}
						cfg, err := clientcmd.NewClientConfigFromBytes([]byte(kubeconfig))
						if err != nil {
							errorResult(p.Spec.NodeName, err.Error())
						}
						rawConfig, err := cfg.RawConfig()
						if err != nil {
							errorResult(p.Spec.NodeName, err.Error())
						}

						var cert string
						var path string
						currentContext := rawConfig.CurrentContext
						user := rawConfig.Contexts[currentContext].AuthInfo
						u := rawConfig.AuthInfos[user]

						if string(u.ClientCertificateData) != "" {
							cert = string(u.ClientCertificateData)
							path = s[1]
						} else if string(u.ClientCertificate) != "" {
							cert, err = ExecPod(coreclient, kubesystemNamespace, &p, []string{"cat", string(u.ClientCertificate)})
							if err != nil {
								errorResult(p.Spec.NodeName, err.Error())
							}
							path = string(u.ClientCertificate)
						} else {
							errorResult(p.Spec.NodeName, err.Error())
						}

						date, days, err := GetDateAndDaysFromCert(cert)
						channel <- serverCertification{
							Entry: Entry{
								Type: "controller-manager",
								Node: p.Spec.NodeName,
								Name: "client-cert",
								Path: path,
								Days: days,
								Due:  date,
							},
							Warning: "",
						}
						break
					}
				}
			}
			mutex.Lock()
			bar.Increment()
			mutex.Unlock()
		}(controllerManagerPod)
	}

	for _, schedulerManagerPod := range schedulerManagerPods.Items {
		wg.Add(1)
		go func(p corev1.Pod) {
			defer wg.Done()
			for _, c := range p.Spec.Containers[0].Command {
				// only for options
				if strings.HasPrefix(c, "--") {
					s := strings.Split(c[2:], "=")
					if s[0] == kubeConfigFlag {
						errorResult := func(node string, errstr string) {
							channel <- serverCertification{
								Entry: Entry{
									Type: "scheduler",
									Node: node,
									Name: "client-cert",
									Path: "-",
									Days: 0,
									Due:  time.Time{},
								},
								Warning: errstr,
							}
						}
						kubeconfig, err := ExecPod(coreclient, kubesystemNamespace, &p, []string{"cat", s[1]})
						if err != nil {
							errorResult(p.Spec.NodeName, err.Error())
						}
						cfg, err := clientcmd.NewClientConfigFromBytes([]byte(kubeconfig))
						if err != nil {
							errorResult(p.Spec.NodeName, err.Error())
						}
						rawConfig, err := cfg.RawConfig()
						if err != nil {
							errorResult(p.Spec.NodeName, err.Error())
						}

						var cert string
						var path string
						currentContext := rawConfig.CurrentContext
						user := rawConfig.Contexts[currentContext].AuthInfo
						u := rawConfig.AuthInfos[user]

						if string(u.ClientCertificateData) != "" {
							cert = string(u.ClientCertificateData)
							path = s[1]
						} else if string(u.ClientCertificate) != "" {
							cert, err = ExecPod(coreclient, kubesystemNamespace, &p, []string{"cat", string(u.ClientCertificate)})
							if err != nil {
								errorResult(p.Spec.NodeName, err.Error())
							}
							path = string(u.ClientCertificate)
						} else {
							errorResult(p.Spec.NodeName, err.Error())
						}

						date, days, err := GetDateAndDaysFromCert(cert)
						channel <- serverCertification{
							Entry: Entry{
								Type: "scheduler",
								Node: p.Spec.NodeName,
								Name: "client-cert",
								Path: path,
								Days: days,
								Due:  date,
							},
							Warning: "",
						}
						break
					}
				}
			}
			mutex.Lock()
			bar.Increment()
			mutex.Unlock()
		}(schedulerManagerPod)
	}

	if o.checkKubelet {
		appClient, err := appsV1Client.NewForConfig(clientConfig)
		if err != nil {
			return err
		}

		dsClient := appClient.DaemonSets(defaultNamespace)
		if err != nil {
			fmt.Println("Exist already")
		} else {
			for {
				time.Sleep(time.Second / 2)

				ds, err := dsClient.Get(name, meta_v1.GetOptions{})
				if err != nil {
					return err
				}
				if ds.Status.NumberAvailable == ds.Status.DesiredNumberScheduled && ds.Status.DesiredNumberScheduled > 0 {
					time.Sleep(time.Second)
					break
				}
			}
		}

		krawlerPods, err := getPods(
			coreclient, defaultNamespace, fmt.Sprintf("app=%s", name))
		if err != nil {
			return err
		}

		for _, p := range krawlerPods.Items {
			wg.Add(1)
			go func(p corev1.Pod) {
				defer wg.Done()
				for i := 1; i <= 10; i++ {
					if p.Status.Phase != corev1.PodRunning {
						time.Sleep(time.Second / 2)
					}
				}
				// p.Spec.NodeName
				var command string
				for i := 1; i <= 5; i++ {
					command, err = ExecPod(
						coreclient,
						defaultNamespace,
						&p,
						[]string{"krawler"},
					)
					if err == nil {
						break
					}
					time.Sleep(time.Second / 2)
				}

				if value, ok := isJSON(command); ok {
					for _, v := range value.Entries {
						warn := ""
						if v.Name == "server-cert" && checkKubeletWithCA == false {
							warn = "Can be ignored this."
						}
						channel <- serverCertification{
							Entry:   v,
							Warning: warn,
						}
					}
				} else {
					channel <- serverCertification{
						Entry: Entry{
							Type: "kubelet",
							Node: p.Spec.NodeName,
							Name: "Error",
							Path: "",
							Days: 0,
							Due:  time.Now(),
						},
						Warning: command,
					}
				}
				mutex.Lock()
				bar.Increment()
				mutex.Unlock()

			}(p)
		}
	}

	go func() {
		defer close(channel)
		wg.Wait()
	}()

	for result := range channel {
		switch v := result.(type) {
		case serverCertification:
			serverCertifications = append(serverCertifications, v)
		default:
			err := fmt.Errorf("Unknown type %T, %+v", result, result)
			log.Error(err)
			return nil
		}
	}

	bar.Finish()
	sort.Slice(serverCertifications, func(i, j int) bool {
		if serverCertifications[i].Entry.Type == "scheduler" && serverCertifications[j].Entry.Type == "kubelet" {
			return true
		}
		if serverCertifications[i].Entry.Type == "kubelet" && serverCertifications[j].Entry.Type == "scheduler" {
			return false
		}
		if serverCertifications[i].Entry.Type < serverCertifications[j].Entry.Type {
			return true
		}
		if serverCertifications[i].Entry.Type > serverCertifications[j].Entry.Type {
			return false
		}
		if serverCertifications[i].Entry.Node < serverCertifications[j].Entry.Node {
			return true
		}
		if serverCertifications[i].Entry.Node > serverCertifications[j].Entry.Node {
			return false
		}
		return serverCertifications[i].Entry.Name < serverCertifications[j].Entry.Name
	})

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Type", "Node", "Name", "Days", "Due", "Path", "Warning"})

	for _, v := range serverCertifications {
		m := []string{v.Entry.Type, v.Entry.Node, v.Entry.Name, cast.ToString(v.Entry.Days), v.Entry.Due.String(), v.Entry.Path, v.Warning}
		table.Append(m)
	}
	table.Render() // Send output

	return nil
}

// ExecPod sets all information required for updating the current context
func ExecPod(coreclient *coreV1Client.CoreV1Client, namespace string, pod *corev1.Pod, command []string) (string, error) {
	req := coreclient.RESTClient().
		Post().
		Namespace(namespace).
		Resource("pods").
		Name(pod.Name).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: pod.Spec.Containers[0].Name,
			Command:   command,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(clientConfig, "POST", req.URL())
	if err != nil {
		return "", fmt.Errorf("%s: %s", req.URL().String(), err.Error())
	}

	var stdout, stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})

	if err != nil {
		return "", fmt.Errorf("%s: %s", strings.Join(command, " "), err.Error())
	}

	if stderr.String() != "" {
		fmt.Println("Error : " + stderr.String())
	}

	return stdout.String(), nil
}

// GetDateAndDaysFromCert takes a cert and extract Date and Days
func GetDateAndDaysFromCert(cert string) (time.Time, int, error) {
	var err error
	timeNow := time.Now()
	block, _ := pem.Decode([]byte(cert))
	if block == nil {
		return time.Time{}, 0, fmt.Errorf("failed to parse certificate PEM")
	}
	c, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("failed to parse certificate: " + err.Error())
	}
	expiresIn := int(c.NotAfter.Sub(timeNow).Hours() / 24)

	return c.NotAfter, expiresIn, err
}
