package main

import (
	"flag"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"ljw/mycontroller/pkg/controller"
	"net"
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
	flag.StringVar(&port, "port", "", "specify kube master port")
	flag.Parse()

	tlsConfig := rest.TLSClientConfig{}
	tlsConfig.CAFile = caFile
	tlsConfig.CertFile = certFile
	tlsConfig.KeyFile = keyFile

	config := rest.Config{
		Host:            net.JoinHostPort(host, port),
		TLSClientConfig: tlsConfig,
	}

	clientSet, err := kubernetes.NewForConfig(rest.AddUserAgent(&config, "my-controller"))
	if err != nil {
		panic(err)
	}

	informerClient, err := kubernetes.NewForConfig(rest.AddUserAgent(&config, "informer"))
	if err != nil {
		panic(err)
	}

	ch := make(chan struct{})
	defer close(ch)
	ctx := wait.ContextForChannel(ch)

	informerFactory := informers.NewSharedInformerFactory(informerClient, 0)
	go controller.NewMyController(informerFactory.Core().V1().Pods(), clientSet).Run(ctx)

	informerFactory.Start(ch)

	<-ch
}
