# kubectl check-cert

`kubectl-check-cert` will help you find the kubernetes certificates that can be expired and check the remaining time.

## How to use

after k8s 1.12 version

    $ kubectl check-cert

or just use below (check name carefully `check_cert` not `check-cert`. See [Names with dashes and underscores](https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/#names-with-dashes-and-underscores))

    $ kubectl-check_cert

and you can also check kubelet certification

    $ kubectl-check_cert --also-check-kubelet

## Example

    $ kubectl-check_cert --also-check-kubelet
    4 / 4 [============================================================] 100.00% 5s
    +--------------------+----------+----------------------------+------+-------------------------------+------------------------------------------------------+----------------------+
    |        TYPE        |   NODE   |            NAME            | DAYS |              DUE              |                         PATH                         |       WARNING        |
    +--------------------+----------+----------------------------+------+-------------------------------+------------------------------------------------------+----------------------+
    | apiserver          | minikube | etcd-certfile              |  354 | 2020-01-10 15:52:33 +0000 UTC | /var/lib/minikube/certs/apiserver-etcd-client.crt    |                      |
    | apiserver          | minikube | kubelet-client-certificate |  354 | 2020-01-10 15:52:31 +0000 UTC | /var/lib/minikube/certs/apiserver-kubelet-client.crt |                      |
    | apiserver          | minikube | proxy-client-cert-file     |  354 | 2020-01-10 15:52:31 +0000 UTC | /var/lib/minikube/certs/front-proxy-client.crt       |                      |
    | apiserver          | minikube | tls-cert-file              |  362 | 2020-01-18 07:29:07 +0000 UTC | /var/lib/minikube/certs/apiserver.crt                |                      |
    | controller-manager | minikube | client-cert                |  362 | 2020-01-18 07:29:10 +0000 UTC | /etc/kubernetes/controller-manager.conf              |                      |
    | scheduler          | minikube | client-cert                |  362 | 2020-01-18 07:29:10 +0000 UTC | /etc/kubernetes/scheduler.conf                       |                      |
    | kubelet            | minikube | client-cert                |  364 | 2020-01-18 07:29:09 +0000 UTC | /etc/kubernetes/kubelet.conf                         |                      |
    | kubelet            | minikube | server-cert                |  356 | 2020-01-10 14:51:39 +0000 UTC | /var/lib/kubelet/pki/kubelet.crt                     | Can be ignored this. |
    +--------------------+----------+----------------------------+------+-------------------------------+------------------------------------------------------+----------------------+

## Install

MacOS

    curl -L https://github.com/leoh0/kubectl-check-cert/releases/download/v0.0.1/kubectl-check_cert_0.0.1_darwin_amd64.tar.gz | tar zxvf - > kubectl-check_cert
    chmod +x kubectl-check_cert
    sudo mv ./kubectl-check_cert /usr/local/bin/kubectl-check_cert

Linux

    curl -L https://github.com/leoh0/kubectl-check-cert/releases/download/v0.0.1/kubectl-check_cert_0.0.1_linux_amd64.tar.gz | tar zxvf - > kubectl-check_cert
    chmod +x kubectl-check_cert
    sudo mv ./kubectl-check_cert /usr/local/bin/kubectl-check_cert

## Explain certification types

### Apiserver

|Type|Name|Explain|
|---------|---|---|
|apiserver|etcd-certfile|apiserver -> etcd client certification|
|apiserver|kubelet-client-certificate|apiserver -> kubelet client certification|
|apiserver|proxy-client-cert-file|front-proxy-client|
|apiserver|tls-cert-file|client -> apiserver server certification|

### Controller manager

|Type|Name|Explain|
|---------|---|---|
|controller-manager|client-cert|controller-manager -> apiserver client certification|

### Scheduler

|Type|Name|Explain|
|---------|---|---|
|scheduler|client-cert| scheduler -> apiserver client certification|

### Kubelet

|Type|Name|Explain|
|---------|---|---|
|kubelet|client-cert| kubelet -> apiserver client certification|
|kubelet|server-cert| apiserver -> kubelet server certification|

## develop

make normal build

    $ make build


make static build

    $ make static

make static linux/amd build

    $ docker run --rm -it -v "$GOPATH":/go -v "$PWD":/app -w /app golang:1.11.5 sh -c 'make release'

## Note

* If you use `--also-check-kubelet` option, then it'll install daemon-set for gathering kubelet information.
* You can safely ignore kubelet's server-cert unless you use the `--kubelet-certificate-authority` option in apiserver. This will appear as a message like `Can be ignored this.`
