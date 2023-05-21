package main

import (
	"context"
	"flag"
	"fmt"
	"net"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	caFile   string
	certFile string
	keyFile  string

	host string
	port string
)

func main() {
	flag.StringVar(&caFile, "ca-file", "", "specify cafile path")
	flag.StringVar(&certFile, "cert-file", "", "specify certfile path")
	flag.StringVar(&keyFile, "key-file", "", "specify keyfile path")
	flag.StringVar(&host, "host", "", "specify kube master ip")
	flag.StringVar(&port, "port", "", "specify kube matser port")
	flag.Parse()

	tlsConfig := rest.TLSClientConfig{}
	tlsConfig.CAFile = caFile
	tlsConfig.CertFile = certFile
	tlsConfig.KeyFile = keyFile

	config := rest.Config{
		Host:            net.JoinHostPort(host, port),
		TLSClientConfig: tlsConfig,
	}

	clientSet, err := kubernetes.NewForConfig(rest.AddUserAgent(&config, "my-cotroller"))
	if err != nil {
		panic(err)
	}

	podsResult, err := clientSet.CoreV1().Pods("kube-system").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err)
	}

	fmt.Printf("namespace\t status\t\t name\n")
	for _, p := range podsResult.Items {
		fmt.Printf("%v\t %v\t %v\n", p.Namespace, p.Status.Phase, p.Name)
	}

	dsResult, err := clientSet.AppsV1().DaemonSets("kube-flannel").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err)
	}

	fmt.Printf("namespace\t name\n")
	for _, d := range dsResult.Items {
		fmt.Printf("%v\t %v\n", d.Namespace, d.Name)
	}
}
