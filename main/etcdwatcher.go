

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/pkg/transport"
	res_apps_v1 "k8s.io/api/apps/v1" // 导入 version 根据 创建的资源version决定
	res_core_v1 "k8s.io/api/core/v1"
	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/kubectl/pkg/scheme"
	"os"
	"os/exec"
)

func main() {
	var endpoint, keyFile, certFile, caFile, script, key string
	var verbose bool
	flag.StringVar(&endpoint, "endpoint", "http://192.168.110.227:2379", "etcd endpoint.")
	flag.StringVar(&keyFile, "cert-key", "", "TLS client key.")
	flag.StringVar(&certFile, "cert", "", "TLS client certificate.")
	flag.StringVar(&caFile, "cacert", "", "Server TLS CA certificate.")
	flag.StringVar(&key, "key", "", " listen this key|key/** change  ")
	flag.StringVar(&script, "script", "", "a hook which be called after event parsed ")
	flag.BoolVar(&verbose, "v", false, "print on the console")
	flag.Parse()
	watch(endpoint, keyFile, certFile, caFile, script, key, verbose)

}

func watch(endpoint, keyFile, certFile, caFile, hook string, key string, verbose bool) {
	tls, err := transport.TLSInfo{
		CertFile:      certFile,
		KeyFile:       keyFile,
		TrustedCAFile: caFile,
	}.ClientConfig()
	if err != nil {
		fmt.Printf("%v", err)
		os.Exit(255)
	}
	config := clientv3.Config{
		Endpoints: []string{endpoint},
		TLS:       tls,
	}
	client, err := clientv3.New(config)
	if err != nil {
		fmt.Println(err)
		os.Exit(255)

	}

	ch := client.Watch(context.TODO(), key, clientv3.WithPrefix())
	parse(ch, hook, verbose)
}

func parse(ch clientv3.WatchChan, hook string, verbose bool) {
	for val := range ch {
		decoder := scheme.Codecs.UniversalDeserializer()
		encoder := jsonserializer.NewSerializer(jsonserializer.DefaultMetaFactory, scheme.Scheme, scheme.Scheme, true)
		for _, msg := range val.Events {

			if msg.IsCreate() {
				fmt.Printf("IsCreate events : %v\n", msg.IsCreate())
			} else if msg.IsModify() {
				fmt.Printf("IsModify events: %v\n", msg.IsModify())
			} else {
				fmt.Println("delete 事件 跳过")
				continue
			}

			kv := msg.Kv
			/*过滤导致出错的key或者自定义过滤的key*/
			switch string(kv.Key) {
			case "compact_rev_key":
				continue
			}
			//调用k8s lib下的序列化struct 解码protobuf数据 封装给runtime.Object(顶层struct)
			obj, gvk, err := decoder.Decode(kv.Value, nil, nil)
			if err != nil {
				fmt.Println(err)
				os.Exit(255)
			}
			//fmt.Printf("--------- group: %v --------- version: %v --------- kind: %v ---------\n",
			//	gvk.Group, gvk.Version, gvk.Kind)

			buff := &bytes.Buffer{}
			err = encoder.Encode(obj, buff)
			if err != nil {
				fmt.Println(err)
				os.Exit(255)
			}
			/*将value 打印在控制台*/
			if verbose {
				if _, err = os.Stdout.Write(buff.Bytes()); err != nil {
					fmt.Println(err)
					os.Exit(255)
				}
			}
			/*区分资源类型进行具体解析*/
			switch gvk.Kind {
			case "Event":
				event, ok := obj.(*res_core_v1.Event)
				if ok {
					EventParse(event, hook)
				}
				break
			case "Node":
				node, ok := obj.(*res_core_v1.Node)
				if ok {
					NodeParse(node, hook)
				}
			case "Pod":
				if gvk.Version != "v1" {
					pod, ok := obj.(*res_core_v1.Pod)
					if ok {
						podParse(pod, hook)
					}
				}
				//截取几个比较重要的信息传入hook作为参数
				break
			case "ReplicaSet":
				if gvk.Version != "v1" {
					rs, ok := obj.(*res_apps_v1.ReplicaSet)
					if ok {
						RsParse(rs, hook)
					}
				}
				break
			case "Endpoints":
				endpoint, ok := obj.(*res_core_v1.Endpoints)
				if ok {
					EndpointParse(endpoint, hook)
				}
				break
			case "Deployment":
				deploy, ok := obj.(*res_apps_v1.Deployment)
				if ok {
					DeployParse(deploy, hook)
				}
				break
			case "Lease":
				//if _, err = os.Stdout.Write(buff.Bytes()); err != nil {
				//	fmt.Println(err)
				//	os.Exit(255)
				//}
			default:
			}
			//podOption:
		}
	}
}

func NodeParse(node *res_core_v1.Node, hook string) {

	fmt.Printf("%v,%v\n", node.Name, node.Status)
}

func EventParse(event *res_core_v1.Event, hook string) {
	fmt.Printf("event name: %v------ reason: %v \n", event.Name, event.Reason)
}

func DeployParse(deploy *res_apps_v1.Deployment, hook string) {

}

func EndpointParse(endpoint *res_core_v1.Endpoints, hook string) {
	fmt.Println("----------", endpoint.Name,endpoint.Name=="")

	for _, subset := range endpoint.Subsets {
		readyIPs := make([]string, len(subset.Addresses))
		for index, address := range subset.Addresses {
			readyIPs[index] = address.IP
		}
		notReadyIPs := make([]string, len(subset.Addresses))
		for index, address := range subset.NotReadyAddresses {
			notReadyIPs[index] = address.IP
		}
		fmt.Printf("-----%v------%v------%v\n", readyIPs, notReadyIPs, subset.Ports)
	}
}

func RsParse(rs *res_apps_v1.ReplicaSet, hook string) {

}

func podParse(pod *res_core_v1.Pod, hook string) {
	fmt.Println("hooks:", hook)
	cmd := exec.Command(hook, pod.Name, pod.Status.PodIP)
	err := cmd.Start()
	if err != nil {
		fmt.Printf("exec hook scripts failed reason : %v\n", err)
	}
}
