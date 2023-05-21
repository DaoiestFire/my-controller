# MyController

#### 介绍
基于client-go实现对k8s集群的访问，这个简易的controller对新创建的pod增加一行annotation

1. v0.1.0: 实现使用证书实例化RESTClient并访问apiserver，打印环境上kube-system命名空间下的pod
2. v0.1.1: 实现使用证书实例化ClientSet并访问apiserver，打印环境上kube-system命名空间下的pod和kube-flannel下的ds

#### 使用教程

##### V0.1
```shell
cd cmd
go mod tidy
go build -o mc
# 指定自己的apiserver的ip与端口，指定证书与密钥
./mc --ca-file /opt/kubernetes/ssl/ca.pem --cert-file /opt/kubernetes/ssl/admin.pem --key-file /opt/kubernetes/ssl/admin-key.pem --host 10.0.4.15 --port 6443
```
可以看到环境上的pod已经打印出来了
![](images/v0.1.png)