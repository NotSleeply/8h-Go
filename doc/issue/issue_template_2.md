## 📌 模块概述

**目标**: 基于《[一个海量在线用户即时通讯系统（IM）的完整设计Plus](https://mp.weixin.qq.com/s/TYUNPgf_3rkBr38rNlEZ2g)》实现 **用户认证与授权系统 (Authentication & Authorization)**，对应文章第1.2.2.1节的 **登录授权(Auth)** 流程和第1.2.2.3节的 **踢人(Kickout)** 机制。

> 📖 **架构参考**: 文章第1.2.2节 - TCP接入核心流程中的 Auth/Logout/Kickout 完整流程。

## ✨ 实现效果

- [ ] 用户可以通过用户名+密码注册账号
- [ ] 登录成功后获得 JWT Token（包含 UserID、过期时间）
- [ ] Token 验证中间件保护所有需要认证的接口
- [ ] 支持多设备登录检测和踢人机制（同类型设备互踢）
- [ ] 登出时清除服务端 Session 和 Redis 缓存
- [ ] 密码使用 bcrypt 加密存储

## 🏗️ 架构定位（对应文章流程）

```
客户端(H5) ──→ Gate(接入层) ──→ Logic(逻辑层) ──→ Storage(存储层)
   │              │                  │                │
   │ ① 登录请求    │ ② 转发验证请求     │ ③ 查询数据库      │ SQLite
   │ (uid+token)  │                  │ 验证用户名密码    │
   │              │                  │                │
   │ ← ④ 返回Token │ ← ⑤ 返回结果      │                │
   │              │                                  │
   │ [Kickout]    │ ⑥ 检查Redis       │                │ Redis
   │              │ 是否其他设备在线   │ (用户状态缓存)   │
```

## 📋 实现步骤

### Step 1: 安装依赖

```bash
go get github.com/golang-jwt/jwt/v5
go get golang.org/x/crypto/bcrypt
```

### Step 2: 实现 JWT 工具类

新建文件: `auth/jwt.go`

```go
package auth

import (
    "github.com/golang-jwt/jwt/v5"
    "time"
)

type Claims struct {
    UserID   string `json:"user_id"`
    UserName string `json:"user_name"`
    jwt.RegisteredClaims
}

var jwtSecret = []byte("your-secret-key") // 后续移至配置文件

func GenerateToken(userID, userName string) (string, error) {
    claims := Claims{
        UserID:   userID,
        UserName: userName,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            NotBefore: jwt.NewNumericDate(time.Now()),
        },
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(jwtSecret)
}

func ParseToken(tokenString string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        return jwtSecret, nil
    })
    if err != nil {
        return nil, err
    }
    if claims, ok := token.Claims.(*Claims); ok && token.Valid {
        return claims, nil
    }
    return nil, err
}
```

### Step 3: 实现认证服务（对应文章Auth流程）

新建文件: `auth/service.go`

```go
package auth

import (
    "context"
    "golang.org/x/crypto/bcrypt"
    "your-project/storage"
)

type AuthService struct{}

// Register 用户注册
func (s *AuthService) Register(ctx context.Context, username, password string) (*storage.User, error) {
    // 1. 检查用户名是否已存在
    existingUser, _ := storage.GetUserByName(username)
    if existingUser != nil {
        return nil, ErrUserAlreadyExists
    }

    // 2. bcrypt 加密密码
    hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    if err != nil {
        return nil, err
    }

    // 3. 创建用户并保存到SQLite
    user := &storage.User{
        UserName:     username,
        PasswordHash: string(hashedPassword),
        NickName:     username,
    }
    if err := storage.CreateUser(user); err != nil {
        return nil, err
    }

    return user, nil
}

// Login 用户登录（对应文章1.2.2.1 Auth流程）
func (s *AuthService) Login(ctx context.Context, username, password string) (string, *storage.User, error) {
    // 1. 从SQLite查询用户
    user, err := storage.GetUserByName(username)
    if err != nil {
        return "", nil, ErrUserNotFound
    }

    // 2. 验证密码
    if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
        return "", nil, ErrInvalidPassword
    }

    // 3. 生成JWT Token
    token, err := GenerateToken(string(rune(user.ID)), user.UserName)
    if err != nil {
        return "", nil, err
    }

    // 4. 【关键】检查是否需要踢出其他设备（对应文章1.2.2.3 Kickout流程）
    // TODO: 在Issue7(Redis集成)完成后实现此步骤
    // checkAndKickoutOtherDevices(ctx, user.ID)

    return token, user, nil
}

// Logout 用户登出（对应文章1.2.2.2 Logout流程）
func (s *AuthService) Logout(ctx context.Context, userID string) error {
    // 1. 清除Redis中的用户Session（TODO: Issue7完成）
    // redis.Del(ctx, fmt.Sprintf("user:session:%s", userID))

    // 2. 更新用户离线状态到SQLite
    return storage.UpdateUserStatus(userID, 0) // 0=离线
}
```

### Step 4: 实现认证中间件（供Gate/API层使用）

新建文件: `auth/middleware.go`

```go
package auth

import (
    "net/http"
    "strings"
)

// JWTMiddleware JWT认证中间件
func JWTMiddleware() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // 1. 从Header获取Token
            authHeader := r.Header.Get("Authorization")
            if authHeader == "" {
                http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
                return
            }

            // 2. 解析 Bearer Token
            tokenString := strings.TrimPrefix(authHeader, "Bearer ")
            claims, err := ParseToken(tokenString)
            if err != nil {
                http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
                return
            }

            // 3. 将用户信息注入Context
            ctx := context.WithValue(r.Context(), ContextKeyUserID, claims.UserID)
            ctx = context.WithValue(ctx, ContextKeyUserName, claims.UserName)

            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

### Step 5: 创建认证相关的HTTP接口（API层预埋）

新建文件: `api/auth_handler.go`

```go
POST /api/v1/auth/register  - 注册
POST /api/v1/auth/login     - 登录
POST /api/v1/auth/logout    - 登出
GET  /api/v1/auth/me        - 获取当前用户信息
```

### Step 6: 编写单元测试

新建文件: `auth/service_test.go`

测试用例:

- 测试正常注册流程
- 测试重复注册失败
- 测试正常登录并返回Token
- 测试错误密码登录失败
- 测试Token解析和过期验证
- 测试bcrypt加密和验证一致性

## 🎯 参考资源

- **📄 架构参考文章**: [《一个海量在线用户即时通讯系统（IM）的完整设计Plus》](https://mp.weixin.qq.com/s/TYUNPgf_3rkBr38rNlEZ2g)
  - 第1.2.2.1节：登录授权(Auth)完整流程图
  - 第1.2.2.2节：登出(Logout)流程
  - 第1.2.2.3节：踢人(Kickout)机制（多设备登录检测）
- **JWT文档**: <https://github.com/golang-jwt/jwt>
- **bcrypt文档**: <https://pkg.go.dev/golang.org/x/crypto/bcrypt>
- **前置依赖**: Issue #1 (SQLite存储层必须先完成)

## 🔍 验收标准

1. ✅ 可以通过 POST `/api/v1/auth/register` 成功注册新用户
2. ✅ 注册时密码已使用 bcrypt 加密存储
3. ✅ 可以通过 POST `/api/v1/auth/login` 成功登录并获得 JWT Token
4. ✅ Token 包含正确的 UserID 和 UserName 信息
5. ✅ Token 有效期为24小时（可配置）
6. ✅ 过期或无效的Token被中间件正确拦截（返回401）
7. ✅ 登出后Token失效（或通过黑名单机制）
8. ✅ 单元测试全部通过 (`go test ./auth/...`)

## ⚠️ 注意事项

- ⚠️ **安全性**: JWT Secret 必须从环境变量或配置文件读取，不能硬编码
- ⚠️ **性能**: bcrypt 的 DefaultCost=10，在注册时会较慢（约100ms），这是正常的
- ⚠️ **扩展性**: 当前版本为简单版JWT，生产环境建议引入 Refresh Token 机制
- ⚠️ **Kickout**: 多设备踢人功能依赖 Redis（将在Issue #7中完善）

## 📊 工作量评估

- 预计耗时: 2天
- 复杂度: ⭐⭐⭐ (涉及安全相关逻辑)
- 依赖:
  - **必须**: Issue #1 (SQLite存储层)
  - **建议**: Issue #3 (配置管理系统) - 用于读取JWT Secret等配置

---
**所属阶段**: 第1周 - 用户认证系统（对应文章1.2.2.1-1.2.2.3节）
**优先级**: P0 (必须完成 - 所有业务的基础)

---

## 设计审查与必要修改（自动追加）

快速要点：

- 登录/踢人流程依赖路由与在线状态，必须从 `Store`/`Redis` 读取或维护路由信息，避免直接使用进程内内存状态导致分布式场景问题。
- Token 与 Session 设计应在 issue 文档里明确：Token 来源、过期/刷新策略、黑名单／登出处理。
- 在实现 Kickout 时应保证消息推送的幂等性与可观察性（记录 kickout 事件以便追溯）。
- 与消息持久化模块协作：登出/踢人事件可能影响未送达消息的重试与死信处理，需在数据库/队列层协同设计。

详见：[ISSUE_DESIGN_REVIEW.md](ISSUE_DESIGN_REVIEW.md)
