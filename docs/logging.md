# 日志开关与脱敏

将以下字段合并到现有配置中，保留原有连接和日志输出配置。修改后重新初始化服务或重启进程生效。

```toml
[log]
sensitive-fields = [] # 没有内置字段；不配置或为空时不脱敏

[[database.redis]]
debug = false # 仅控制这个 Redis 连接的 Debug 日志
```

Redis 的每个 `[[database.redis]]` 连接独立设置开关。Go 代码直接初始化时设置 `Redis.Debug`。手动注册钩子时设置 `LogHook{Debug: true}`。

## 默认行为

- Redis 的 `debug` 默认为 `false`，关闭普通命令及 Pipeline 的 Debug 日志。设置 `true` 后按原格式记录 `args` 和 `cmd`，包含命令参数及返回值。
- Redis 的 Error 日志不受 `debug` 影响，保留原来的参数、错误内容和错误忽略规则。连接初始化日志也不受影响。
- GORM 不增加配置开关，使用原生 `.Debug()` / `Logger.LogMode(...)`。默认级别改为 `Warn`，普通查询不打印 Debug，慢查询和错误仍打印；调用 `Db.Debug()` 后打印当前操作的 SQL。需要彻底关闭 GORM 日志时使用原生 `logger.Silent`。
- 日志开关和脱敏互相独立。没有配置 `sensitive-fields` 时，SQL 参数、Redis 参数及返回值、错误消息均不作脱敏。

```go
// 当前操作开启 SQL 调试日志
service.Db.Debug().Where("id = ?", id).First(&user)

// 当前会话关闭普通查询日志，仍保留慢查询和错误
db := service.Db.Session(&gorm.Session{
    Logger: service.Db.Logger.LogMode(logger.Warn),
})
```

## 可配置字段

`sensitive-fields` 是完整的脱敏字段列表，**没有任何内置字段**。例如只配置 `["phone"]` 时，`phone` 会脱敏，未配置的 `password`、`token` 都保持原值。

字段规则匹配任意层级的日志字段名，忽略大小写、下划线和连字符。例如 `id_card` 同时匹配 `idCard`、`ID-Card`。不支持通配符或字段路径。经 `InitLogger` 创建的日志器会把规则应用到各级别的文件和标准输出。

命中字段的整个值替换成 `[REDACTED]`。嵌套对象、数组、结构体、`With(...)` 预绑定字段和包含 JSON 的字符串均会处理。只修改日志副本，不修改请求内容或数据库操作数据。

```json
{"body":{"name":"张三","phone":"[REDACTED]","password":"未配置则保留原值"}}
```

GORM 输出的 SQL 是日志中的 `sql` 字符串；Redis 输出包含 `args` 数组和 `cmd` 字符串。字段规则不解析 SQL 列名、Redis 的位置参数或拼接的命令文本。因此配置 `"password"` 只处理结构化数据中的同名字段，不会自动遮盖 SQL 或 Redis 命令文本中的密码。

如果需要隐藏这些完整日志字段，显式配置：

```toml
[log]
sensitive-fields = ["password", "token", "phone", "sql", "args", "cmd", "error"]
```

规则也不解析任意自由文本或 URL 查询串；可按需列入 `"query"`、`"url"` 或 `"msg"` 来隐藏整个对应字段。
