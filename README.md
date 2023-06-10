# MyController

#### 介绍

基于client-go实现对k8s集群的访问，实现了一个简易的controller

1. v0.1.0: 实现使用证书实例化RESTClient并访问apiserver，打印环境上kube-system命名空间下的pod
2. v0.1.1: 实现使用证书实例化ClientSet并访问apiserver，打印环境上kube-system命名空间下的pod和kube-flannel下的ds
3. master: 实现一个简易的controller。
- 如果一个pod包含标签"mycontroller:mycontroller"，就为pod增加一个新label"processed:processed"
- 如果更新已存在pod内容，删除了标签"mycontroller:mycontroller"就需要将"processed:processed"标签清除
- 如果更新已存在pod内容，增加了标签"mycontroller:mycontroller"就需要将"processed:processed"标签添加

#### 使用教程

```shell
cd cmd
go mod tidy
go build -o mc
# 指定自己的apiserver的ip与端口，指定证书与密钥
./mc --ca-file /opt/kubernetes/ssl/ca.pem --cert-file /opt/kubernetes/ssl/admin.pem --key-file /opt/kubernetes/ssl/admin-key.pem --host 10.0.4.15 --port 6443
```

##### master




##### V0.1.1

可以看到环境上的pod与ds已经打印出来了
![](images/v0.1.1.png)

##### V0.1.0

可以看到环境上的pod已经打印出来了
![](images/v0.1.png)
