# InitGin 的客户端 IP 默认值

`service.InitGin` 创建 Gin Engine 时默认不信任任何代理，且平台头快捷通道默认关闭。
未显式配置可信代理时，`c.ClientIP()` 返回连接来源 IP。经过 Ingress 时，这通常是
Ingress 到 Go 的 TCP 对端地址。

公共配置提供 `server.trusted-proxies` 和 `server.remote-ip-headers`：

```toml
[server]
# 只填直接连接 Go 的 Ingress/反向代理可能使用的来源网段。
# 留空时不信任默认的 X-Forwarded-For / X-Real-IP。
trusted-proxies = ["10.20.0.0/24", "fd00:10:20::/64"]

# 默认是 ["X-Forwarded-For", "X-Real-IP"]；仅特殊网关头需要配置。
remote-ip-headers = ["X-CDN-Client-IP"]
```

配置会在 `InitGin` 创建 Engine 后、注册路由前应用，因此同时影响访问日志和业务的
`ClientIP()`。配置错误会记录初始化错误并停止启动流程。

默认头使用 `X-Forwarded-For` 和 `X-Real-IP`。这覆盖常见云负载均衡与 Nginx Ingress
写法，但不承诺覆盖所有云产品、自建网关或特殊 Ingress 注解。`remote-ip-headers`
可按实际部署改为网关写入的客户端 IP 头；无论使用哪个头，都必须同时配置
`trusted-proxies`，否则 Gin 不会信任该头。

`X-Forwarded-For` 按 Gin 的信任链从右向左解析：只有链上的来源都落在
`trusted-proxies` 中，才会继续取更左侧的客户端地址。对于
`CLB/ALB → Ingress → Go`，通常至少要信任 Ingress 到 Go 的实际来源网段；如果
Ingress 继续转发上游代理地址，也要把链中会出现的代理网段一起列入。不要把云厂商
入口地址、`0.0.0.0/0` 或整个内网作为猜测值。

项目仍可以在路由初始化回调中显式调用 Gin 配置方法覆盖公共配置，但建议把部署差异
放进配置文件或环境变量，而不是写死在公共库。

这是一次有意收紧的运行行为变更。升级公共库之前，所有经代理访问且依赖客户端 IP 的
项目都应显式配置自己的可信代理，否则日志和白名单会使用代理的连接来源 IP。本地
修改不会自动升级其他项目依赖，也不会改变已部署的服务。

填写实际回源代理 IP/CIDR，不要使用负载均衡的入口地址猜测回源地址，不要默认信任
所有 IPv4/IPv6 或整个内网。公共库不预置任何云厂商或集群网段。