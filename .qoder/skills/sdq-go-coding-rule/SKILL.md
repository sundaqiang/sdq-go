---
name: sdq-go-coding-rule
description: sdq-go 项目编码规范与架构约定。编写或修改 Go 代码时自动应用。
---

# sdq-go 项目规则

## 项目结构

```
├── main.go              # 入口：初始化配置 + 启动Gin
├── service/             # 框架封装（Gin/GORM/Redis/Zap/Mongo/FastHttp 等）
├── common/              # 纯函数工具（不依赖项目状态）
├── router/              # 路由注册（按模块组织 Group）
├── handler/             # 接口入口（参数绑定 + 调用logic + 输出响应）
├── logic/               # 业务逻辑（DB/Redis/外部调用）
├── model/               # 数据模型结构体
├── config.toml          # 配置
└── logs/                # 日志输出
```

### 分层职责

| 层 | 职责 | 规则 |
|------|------|------|
| `router` | 路由注册 | 只做 Group + 路径绑定，不写逻辑 |
| `handler` | 参数绑定、调用logic、输出响应 | 保持精简，不包含DB/Redis操作 |
| `logic` | 业务运算、数据存取 | 通过 interface 定义契约，便于 mock 测试 |
| `model` | 结构体定义 | 无依赖，纯数据描述 |
| `service` | 框架封装、初始化 | 不写业务逻辑 |
| `common` | 通用工具函数 | 纯函数，不依赖项目状态 |

### 典型调用流程

```
router → handler → logic → model
                  ↓
              service(Tracer)
```

## 编码风格

### 命名

| 元素 | 风格 | 示例 |
|------|------|------|
| 包名 | 小写单词 | `handler`、`logic` |
| 公开符号 | 大驼峰 | `InitGin`、`GetAccount` |
| 私有符号 | 小驼峰 | `initDB`、`getEncoder` |
| 常量 | 驼峰 | `redisPrefix` |

### 注释

- 使用中文
- 公开函数：`// FuncName 功能描述`
- 多参数/复杂逻辑：`/* ... */` 块注释

### 结构体标签

```go
type Example struct {
    MaxSize  int    `toml:"max-size" json:"max_size"`  // toml=kebab-case, json/form/gorm=snake_case
    UserName string `form:"user_name" gorm:"column:user_name" binding:"required" msg:"用户名不能为空"`
}
```

### 代码精简

- 逻辑能合并则合并，不为拆分而拆分
- 善用泛型消除类型重复
- 优先复用 `common` 包工具函数
- 不随意引入新依赖，优先用已有库解决

## 技术选型

| 场景 | 库 |
|------|------|
| Web | `gin-gonic/gin` |
| ORM | `gorm.io/gorm`（MySQL/SQLite） |
| Redis | `go-redis/v9` |
| 本地缓存 | `orca-zhang/ecache` |
| 日志 | `zap` + `lumberjack` |
| 配置 | `koanf/v2`（TOML + 环境变量） |
| JSON | `bytedance/sonic`（禁止 `encoding/json`） |
| HTTP客户端 | `valyala/fasthttp` |
| 定时任务 | `gocron/v2` |
| 参数校验 | `validator/v10` |
| MongoDB | `mongo-driver` |
| 分布式ID | `sonyflake` |

## 架构约定

### 初始化

- 每个模块独立 `Init*` 函数，`InitConfig()` 按序统一调用
- 顺序：日志 → DB → Redis → Mongo → FastHttp → Cron → SonyFlake → Limiter → Cache
- 失败即 `Fatal`，不允许带病运行

### Tracer 上下文

所有业务代码通过 Tracer 获取依赖，不直接访问全局变量：

```go
// HTTP请求内
t := service.GetGinTracer(c)

// 非HTTP场景（定时任务/后台逻辑）
t := service.GetGeneralTracer()
```

### 全局实例

集中声明在 `service/service.go` 的 `var` 块，新增组件同样加入此处。

## 接口规范

### handler 写法

handler 只做三件事：绑定参数、调用logic、返回响应：

```go
// handler/pay.go
func Ping(c *gin.Context) {
    t := service.GetGinTracer(c)
    l := logic.NewPayLogic(t)
    data, err := l.GetAccount(1)
    if err != nil {
        t.GetHttpResFailure(http.StatusOK, 40001, "查询失败")
        return
    }
    t.GetHttpResSuccess(http.StatusOK, 10000, data)
}
```

### logic 写法

logic 通过 interface 定义契约，实现类接收 Tracer：

```go
// logic/pay.go

// PayLogic 支付模块业务接口
type PayLogic interface {
    GetAccount(id int64) (*model.WxOfficeAccount, error)
}

type payLogic struct {
    t *service.GinTracer
}

// NewPayLogic 创建支付模块实例
func NewPayLogic(t *service.GinTracer) PayLogic {
    return &payLogic{t: t}
}

func (l *payLogic) GetAccount(id int64) (*model.WxOfficeAccount, error) {
    var account model.WxOfficeAccount
    err := l.t.Db.Where("id = ?", id).First(&account).Error
    return &account, err
}
```

### Interface 规范

- **只在 logic 层定义**，不在 handler/service 层泛滥使用
- **接口跟随模块**：每个业务模块一个 interface（`PayLogic`、`UserLogic`）
- **小接口原则**：每个 interface 只包含该模块的方法，不做大而全
- **命名**：`XxxLogic`（interface）+ `xxxLogic`（实现）+ `NewXxxLogic`（构造）
- **用途**：团队協作时明确契约；单元测试时 mock 替换

### 路由注册

```go
// router/router.go
func InitRouter(r *gin.Engine) {
    pay := r.Group("/pay")
    {
        pay.GET("/ping", handler.Ping)
        pay.POST("/wechat", handler.Wechat)
    }
}
```

### 统一响应

```go
// 成功：{"success":true, "code":10000, "trace_id":"...", "data":{...}}
t.GetHttpResSuccess(http.StatusOK, 10000, data)

// 失败：{"success":false, "code":40001, "trace_id":"...", "msg":"..."}
t.GetHttpResFailure(http.StatusOK, 40001, "原因")

// 参数错误（自动翻译）
t.GetHttpResErrorTrans(http.StatusOK, 40002, err)
```

### 请求绑定

使用 `t.BindJson(code, &body)` / `t.BindForm(...)` / `t.BindQuery(...)`，失败自动返回错误响应。

## 日志

- 通过 `t.Log` 输出，自动携带 TraceID
- 级别：`Debug` → `Info` → `Warn` → `Error` → `Fatal`
- 格式：JSON，按级别分文件
- 禁止 `fmt.Println`

## 配置

- 格式：TOML，键名 kebab-case
- 环境变量覆盖：`InitConfig("config.toml", "APP_", &conf)`
- 可选模块用指针字段（`*Server`），nil 时跳过初始化

## 错误处理

- 初始化失败：`ZapLog.Fatal("描述", zap.Error(err))`
- 业务逻辑：逐层返回 `error`，handler 层统一决策响应
- 参数校验：框架自动翻译为中文
