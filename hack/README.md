Add as a sidecar:

```shell
kubectl patch deployment you-app -p'{"spec":{"template":{"spec":{"containers":[{"name":"hd","image":"ghcr.io/linuxsuren/hd","command":["/bin/sh"],"args":["-c","while true; do echo hello; sleep 10;done"]}]}}}}'
```
