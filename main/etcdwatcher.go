// Copyright 2017 The casbin Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/pkg/transport"
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
				fmt.Printf("IsCreate events : %v", msg.IsCreate())
			} else if msg.IsModify() {
				fmt.Printf("IsModify events: %v", msg.IsModify())
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
			fmt.Printf("--------- group: %v --------- version: %v --------- kind: %v ---------\n",
				gvk.Group, gvk.Version, gvk.Kind)

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
			case "Pod":
				//截取几个比较重要的信息传入hook作为参数
				podParse(err, buff, hook)
			case "ReplicaSet":

			case "Endpoint":

			case "Deployment":
			case "Lease":
				if _, err = os.Stdout.Write(buff.Bytes()); err != nil {
					fmt.Println(err)
					os.Exit(255)
				}
			default:
			}
			//podOption:
		}
	}
}

func podParse(err error, buff *bytes.Buffer, hook string) {
	var jsonMap map[string]interface{}
	err = json.Unmarshal(buff.Bytes(), &jsonMap)
	if err != nil {
		fmt.Println(err)
		os.Exit(255)
	}
	podStatus := jsonMap["status"].(map[string]interface{})
	podMetadata := jsonMap["metadata"].(map[string]interface{})
	fmt.Printf("podName:%v--------podIP:%v----------podState:[", podMetadata["name"], podStatus["podIP"])

	podConditions, ok := podStatus["conditions"].([]interface{})
	if ok {
		for _, conditionInterface := range podConditions {
			condition := conditionInterface.(map[string]interface{})
			fmt.Printf("type:%v----status:%v,", condition["type"], condition["status"])
		}
	}
	fmt.Printf("],")
	podContainerStatuses, ok := podStatus["containerStatuses"].([]interface{})
	if ok {
		for _, containerStatusInterface := range podContainerStatuses {
			containerStatus := containerStatusInterface.(map[string]interface{})
			containerState := containerStatus["state"].(map[string]interface{})
			for key, value := range containerState {
				optionState := value.(map[string]interface{})
				fmt.Printf("%v:{%v}", key, optionState)
			}

			fmt.Printf("isReady:%v-----image:%v", containerStatus["ready"], containerStatus["image"])
		}

	}
	fmt.Printf("]\n")
	fmt.Println("*******************************************************************************************************************")
	podName, ok := podMetadata["name"].(string)
	if !ok {
		podName = "nil"
	}
	podIP, ok := podStatus["podIP"].(string)
	if !ok {
		podIP = "nil"
	}
	fmt.Println("hooks:", hook)
	cmd := exec.Command(hook, podName, podIP)
	err = cmd.Start()
	if err != nil {
		fmt.Printf("exec hook scripts failed reason : %v\n", err)
	}
}
