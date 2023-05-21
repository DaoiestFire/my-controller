package main

import (
	"context"
	"flag"
	"fmt"
	"net"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
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

	config.APIPath = "api"
	config.GroupVersion = &corev1.SchemeGroupVersion
	config.NegotiatedSerializer = scheme.Codecs

	rest.AddUserAgent(&config, "my-controller")

	restClient, err := rest.RESTClientFor(&config)
	if err != nil {
		panic(err)
	}

	result := &corev1.PodList{}

	err = restClient.Get().
		Namespace("kube-system").
		Resource("pods").
		VersionedParams(&metav1.ListOptions{Limit: 100}, scheme.ParameterCodec).
		Do(context.TODO()).Into(result)
	if err != nil {
		panic(err)
	}

	fmt.Printf("namespace\t status\t\t name\n")
	for _, d := range result.Items {
		fmt.Printf("%v\t %v\t %v\n", d.Namespace, d.Status.Phase, d.Name)
	}
}
