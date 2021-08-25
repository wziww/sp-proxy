# sp-proxy
./sp-client -c conf.toml
```shell
# routers 配置按顺序执行，匹配到一个后不会进行后续匹配
# routers.upstream 代理转发到的后端服务地址
# routers.path 正则，用来匹配 req.URL
# routers.strip 匹配到后，是否去除路径的前 N 位地址。如 /api/test  strip = 4 则转发的路径为 /test
# routers.copy_stream 流量拷贝，功能和 upstream 类似，两者互不影响，便于本地调试
```
