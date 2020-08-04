module etcd-k8s-watcher

go 1.13

replace (
	github.com/coreos/bbolt v1.13.5 => go.etcd.io/bbolt v1.13.5
	go.etcd.io/bbolt v1.13.5 => github.com/coreos/bbolt v1.13.5
	go.etcd.io/etcd v0.4.9 => github.com/etcd-io/etcd v3.4.9+incompatible
)

require (
	go.etcd.io/etcd v0.4.9
	k8s.io/apimachinery v0.18.6
	k8s.io/kubectl v0.18.6
)
