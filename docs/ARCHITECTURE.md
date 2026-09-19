# 轴承首末检系统 V1 系统架构说明

> 文档版本：V1.0
> 编制日期：2026-09-19
> 适用范围：当前仓库 D:\\code\\DG 的初版系统，包括 PC 管理端、PDA 现场端、Go API、PostgreSQL 数据库、日志导出、审计与备份恢复。

## 1. 文档目的

本文从业务、应用、数据、权限、部署和运维六个视角说明系统架构，重点回答以下问题：

1. 系统由哪些模块组成，各模块如何协作。
2. 首检、末检、报表和登记表的核心业务链路如何运行。
3. 多部门、角色、权限和产线范围如何隔离。
4. 表单模板、版本、产线分配和填报快照如何保证可追溯。
5. 系统如何构建、启动、备份、恢复和扩展。

## 2. 系统定位

系统部署在工厂局域网内，面向轴承首末检及现场过程记录场景。主要目标：

- 将首检、末检、报表、登记表从纸质记录转为结构化电子记录。
- 由班次、产线、表单版本和操作人员共同确定填报上下文。
- 提供按日期、产线、模板、填报人检索的历史日志与 Excel 导出。
- 对废除、恢复、模板发布、配置修改等关键动作保留审计记录。
- 支持多部门数据隔离。
- 为后续电子仪表和 MQTT 自动采集预留扩展接口。

## 3. 总体架构

### 3.1 逻辑架构

~~~mermaid
flowchart LR
    U1[PC 管理员/配置人员] --> PC[Vue 3 PC 端 Element Plus]
    U2[PDA 现场操作人员] --> PDA[Vue 3 PDA 端 Vant]
    PC -->|HTTP / JSON / Cookie| API[Go + Gin API]
    PDA -->|HTTP / JSON / Cookie| API
    API --> AUTH[JWT / bcrypt 认证服务]
    API --> RBAC[RBAC 权限与部门/产线范围]
    API --> SVC[表单、班次、备份、导入导出服务]
    SVC --> DB[(PostgreSQL 18)]
    SVC --> DUMP[pg_dump 完整备份]
    SVC --> ZIP[部门 ZIP 归档]
    MQTT[未来 MQTT/电子仪表] -. InstrumentIngestor 扩展点 .-> API
    API --> STATIC[静态资源 /pc /pda]
~~~

### 3.2 部署拓扑

~~~mermaid
flowchart TB
    BrowserPC[PC 浏览器] -->|http://服务器:8080/pc/| HTTP[Go HTTP 服务]
    BrowserPDA[PDA 浏览器] -->|http://服务器:8080/pda/| HTTP
    HTTP --> API[Go API / Gin]
    API --> PG[(PostgreSQL)]
    API --> FS[(backups 目录)]
    PG --> PGDUMP[pg_dump 可执行文件]
~~~

API 程序同时承担前端静态文件服务：web-pc/dist 挂载到 /pc，web-pda/dist 挂载到 /pda，根路径重定向到 /pc/。

### 3.3 运行端口

| 环境 | 组件 | 默认地址 |
|---|---|---|
| 开发 | API | http://127.0.0.1:8080 |
| 开发 | PC Vite | http://127.0.0.1:5173/pc/ |
| 开发 | PDA Vite | http://127.0.0.1:5174/pda/ |
| 生产/联调 | API + 静态端 | http://服务器:8080/pc/、/pda/ |
| 健康检查 | API | GET /health |

## 4. 技术栈

| 层次 | 技术 | 说明 |
|---|---|---|
| 后端语言 | Go 1.27 | 单二进制 API 服务，便于内网部署 |
| HTTP 框架 | Gin | 路由、中间件、请求绑定、静态资源 |
| ORM | GORM | 模型映射、事务、软删除、预加载 |
| 数据库 | PostgreSQL 18 | JSONB、部分唯一索引、事务和触发器 |
| 认证 | JWT HS256 + bcrypt | 登录凭证和密码散列 |
| 数据库迁移 | golang-migrate | server/migrations 顺序迁移 |
| Excel | Excelize | 导入模板、批量导入、日志导出 |
| 备份 | pg_dump / ZIP / JSON | 完整数据库备份、部门业务归档 |
| PC 前端 | Vue 3 + TypeScript + Vite + Element Plus + Pinia | 管理、配置、日志和备份 |
| PDA 前端 | Vue 3 + TypeScript + Vite + Vant + Pinia | 现场填报和本人历史 |
| 本地草稿 | IndexedDB | PDA 断网时保留未提交草稿 |

## 5. 代码结构

~~~text
D:\\code\\DG
├─ server/                         Go 后端
│  ├─ cmd/
│  │  ├─ api/                      API 服务入口
│  │  ├─ dev/                      一键开发启动器
│  │  ├─ migrate/                  数据库迁移命令
│  │  ├─ seed/                     初始化权限、角色、默认部门和管理员
│  │  └─ reset-admin/              admin 密码重置命令
│  ├─ internal/
│  │  ├─ auth/                     JWT 与密码散列
│  │  ├─ config/                   环境变量配置
│  │  ├─ database/                 GORM/PostgreSQL 连接
│  │  ├─ model/                    领域模型
│  │  ├─ rbac/                     权限目录与授权器
│  │  ├─ service/                  表单、班次、种子、备份
│  │  │  ├─ backup/                完整备份、部门归档、恢复
│  │  │  └─ instrument/            仪表接入扩展接口
│  │  └─ httpapi/                  Gin 路由、中间件、处理器、范围控制
│  └─ migrations/                  PostgreSQL 迁移脚本
├─ web-pc/                         PC 管理端
├─ web-pda/                        PDA 现场端
├─ packages/shared/                共享 TypeScript 类型与格式化函数
├─ scripts/                        PowerShell 构建、开发、迁移、停止脚本
└─ README.md                       项目快速启动说明
~~~

## 6. 后端分层设计

### 6.1 启动层 server/cmd

- api：加载配置、连接数据库、装配认证/RBAC/备份/仪表服务、注册路由、监听 HTTP、处理退出信号。
- dev：按顺序执行迁移、初始化，再启动 API、PC 和 PDA 开发服务器，并在 Ctrl+C 时清理子进程。
- migrate：执行 up、down 或指定步数迁移。
- seed：幂等写入权限目录、内置角色、默认部门和默认超级管理员。
- reset-admin：运维场景下重置 admin 密码。

### 6.2 配置与数据库层

配置来源为环境变量及 server/.env：

| 配置项 | 用途 |
|---|---|
| APP_ENV | 运行环境；生产模式下 Cookie 启用 Secure |
| HTTP_ADDR | API 监听地址，默认 :8080 |
| DATABASE_URL | PostgreSQL 连接串 |
| TEST_DATABASE_URL | 测试数据库连接串 |
| JWT_SECRET | JWT 签名密钥 |
| JWT_TTL | 登录有效期，默认 12 小时 |
| PG_DUMP_PATH | pg_dump.exe 路径 |
| BACKUP_DIR | 备份文件目录 |

数据库访问统一通过 GORM。模型使用软删除字段 deleted_at，删除业务对象时保留历史引用。

### 6.3 领域模型层 server/internal/model

核心模型分为以下领域：

1. 组织与权限：Department、User、Role、Permission、RolePermission、RoleLineScope。
2. 基础配置：Line、Product、ShiftPlan、ShiftPlanItem。
3. 动态表单：FormTemplate、FormVersion、LineFormAssignment。
4. 填报数据：Submission、SubmissionRow、FieldValue。
5. 系统能力：AuditLog、InstrumentMessage、AppSetting、DepartmentSetting、BackupRecord。

### 6.4 认证与授权层

认证流程：

1. PC 登录需要账号和密码，密码使用 bcrypt 校验。
2. PDA 登录只需要工号；账号必须启用、允许 PDA、所属部门可用且拥有 pda.login 权限。
3. 登录成功后签发 HS256 JWT，同时写入 HttpOnly Cookie。客户端可使用 Cookie 或 Authorization: Bearer。
4. 受保护接口通过 Auth 中间件解析 JWT，将 user_id 和客户端类型写入 Gin Context。
5. RequirePermission 校验权限码；业务处理器再结合部门范围和产线范围判断数据可见性。

授权模型包含四层：

- 角色权限：role_permissions 决定角色能做哪些动作。
- 客户端权限：PC 和 PDA 使用不同的登录权限，接口同时检查客户端类型。
- 部门范围：超级管理员可切换部门或查看全部；普通账号固定绑定一个部门。
- 产线范围：自定义角色通过 role_line_scopes 可限制到指定产线；内置管理员和普通用户默认可访问本部门全部启用产线。

### 6.5 业务服务层

- 表单服务：解析/校验 JSON Schema、解析默认变量、检查产品字段、生成新版本。
- 班次服务：计算当前班次和业务日期，支持跨零点班次。
- 种子服务：保证权限、内置角色、默认部门和超级管理员幂等存在。
- 备份服务：调用 pg_dump 生成完整备份，或生成部门 ZIP 归档。
- 恢复服务：解析部门 ZIP，在校验和数据库事务中覆盖目标部门数据。
- 仪表服务：定义 Ingestor 接口，当前只实现数据库落库，MQTT 接入尚未启用。

### 6.6 HTTP 接口层

接口统一使用 /api/v1 前缀，分为：

- 公共接口：登录、健康检查、品牌信息。
- 已登录接口：当前用户、退出、废除原因。
- PC 管理接口：/admin/*。
- 执行接口：/execution/*，PC 和 PDA 共用。

关键中间件：

| 中间件 | 作用 |
|---|---|
| RequestID | 为请求生成/透传 X-Request-ID，便于日志关联 |
| Auth | 解析 JWT，写入用户上下文 |
| RequireClient | 限制 PC 管理接口只能由 PC 登录 Token 调用 |
| RequirePermission | 校验权限码 |
| CORS | 开发环境允许带凭证的跨域请求 |

## 7. 前端架构

### 7.1 PC 管理端

PC 端使用 Element Plus，适合表格、表单、弹窗和批量配置。主要页面如下：

| 路由 | 页面 | 主要职责 |
|---|---|---|
| / | 工作台 | 当前用户、可访问产线、快捷入口 |
| /execution | PC 填报 | 按产线、任务和班次填报 |
| /users | 用户与权限 | 用户、角色分配、密码重置、Excel 导入 |
| /departments | 部门维护 | 部门新增、启停、删除 |
| /lines | 产线配置 | 两级产线维护、Excel 导入 |
| /products | 产品维护 | 产品与产线关联 |
| /shifts | 班次配置 | 班次计划和跨零点配置 |
| /forms | 任务与表单 | 动态表单设计、版本、发布、复制 |
| /assignments | 产线分配 | 模板版本分配到产线 |
| /logs | 日志与导出 | 填报查询、日历、废除、恢复、Excel |
| /backups | 数据库备份 | 完整备份、部门归档、恢复 |
| /void-reasons | 废除原因 | 维护废除原因选项 |
| /settings | Logo 设置 | 品牌 Logo 和部门级设置 |

前端状态要点：

- auth Pinia Store 保存用户、权限、可访问产线、部门和品牌信息。
- 超级管理员在顶部选择部门后，将部门 ID 写入 localStorage，Axios 每次请求带 X-Department-ID。
- 后端始终以 Token 和数据库中的真实用户为准，不信任前端传入的部门字段。
- 前端权限只负责隐藏菜单和按钮，最终授权由后端完成。

### 7.2 PDA 现场端

PDA 端使用 Vant，面向触摸屏和移动浏览器。页面包括：

- 登录：支持记住工号、自动登录；不保存密码。
- 工作台：检测在线状态、选择产线、查看可填写任务。
- 表单填报：动态渲染字段、产品选择、数字上下限提示、本地草稿、在线提交。
- 我的历史：分页查询本人明细，查看状态、上下限异常、废除和恢复。

PDA 离线策略：

- IndexedDB 按用户 ID、表单版本 ID 和产线 ID 保存草稿。
- 在线时允许提交、历史查询、废除和恢复。
- 离线时只允许继续编辑和保存草稿，不能提交或执行服务端操作。

## 8. 数据架构

### 8.1 主要关系

~~~mermaid
erDiagram
    DEPARTMENT ||--o{ USER : contains
    DEPARTMENT ||--o{ LINE : contains
    DEPARTMENT ||--o{ PRODUCT : contains
    DEPARTMENT ||--o{ SHIFT_PLAN : contains
    DEPARTMENT ||--o{ FORM_TEMPLATE : contains
    DEPARTMENT ||--o{ SUBMISSION : owns
    ROLE ||--o{ USER : grants
    ROLE ||--o{ ROLE_PERMISSION : has
    PERMISSION ||--o{ ROLE_PERMISSION : granted
    ROLE ||--o{ ROLE_LINE_SCOPE : limits
    LINE ||--o{ LINE : parent_child
    PRODUCT }o--o{ LINE : product_lines
    SHIFT_PLAN ||--o{ SHIFT_PLAN_ITEM : contains
    FORM_TEMPLATE ||--o{ FORM_VERSION : versions
    LINE ||--o{ LINE_FORM_ASSIGNMENT : assigned
    FORM_TEMPLATE ||--o{ LINE_FORM_ASSIGNMENT : assigned
    FORM_VERSION ||--o{ LINE_FORM_ASSIGNMENT : active
    SUBMISSION ||--o{ SUBMISSION_ROW : rows
    SUBMISSION_ROW ||--o{ FIELD_VALUE : values
~~~

### 8.2 核心表说明

| 表 | 说明 |
|---|---|
| departments | 部门/分厂，业务数据隔离边界 |
| users | 用户账号、部门、角色和密码散列 |
| roles | 内置和自定义角色，控制 PC/PDA 登录 |
| permissions | 系统权限目录 |
| role_permissions | 角色与权限的多对多关系 |
| role_line_scopes | 自定义角色的产线访问范围 |
| lines | 两级产线树，部门内编码唯一 |
| products | 产品基础数据 |
| product_lines | 产品与可用产线关联 |
| shift_plans | 班次计划，按生效时间选择最新计划 |
| shift_plan_items | 班次明细，保存开始/结束时间和跨零点标志 |
| form_templates | 动态表单模板，首末检/报表/登记表 |
| form_versions | 模板版本，保存 JSONB Schema 和状态 |
| line_form_assignments | 产线当前启用的模板版本 |
| submissions | 一次填报主记录，保存部门和班次快照 |
| submission_rows | 填报明细行，保存操作人和废除/恢复状态 |
| field_values | 字段值，支持文本、数字、多选和重复值 |
| audit_logs | 审计记录，保存前后 JSON、操作人、IP、请求 ID |
| instrument_messages | 仪表原始报文，V1 预留 |
| app_settings | 全局设置 |
| department_settings | 部门级 Logo、废除原因等设置 |
| backup_records | 完整备份和部门归档记录 |

### 8.3 软删除和唯一性

系统对部门、用户、角色、产线、产品、模板和填报主记录使用软删除。数据库通过部分唯一索引保证未删除数据中的编码唯一，例如：

- 部门编码在未删除数据中唯一。
- 同一部门内产线编码唯一。
- 同一部门内产品编码唯一。
- 同一部门内模板编码唯一。
- 用户账号在未删除数据中唯一。

这样可以重新使用历史已删除对象的编码，同时保留历史引用。

### 8.4 表单 Schema

form_versions.schema 保存 JSON Schema。示例字段包括 key、label、type、required、unit、min、max、allowMultiple、repeatable、sortOrder。

支持的字段类型：

| 类型 | 用途 |
|---|---|
| text | 文本 |
| number | 数值，可配置上下限和单位 |
| single_select | 单选 |
| multi_select | 多选 |
| datetime | 日期时间 |
| static_text | 固定说明文本 |
| product | 从当前产线关联产品中选择 |

默认变量包括 current_date、current_time、current_shift、user_name、user_account、line_name、line_product、form_name、stage。前端使用双花括号语法解析这些变量。

### 8.5 版本化与快照

- 模板修改不会覆盖历史版本，而是创建新的 form_versions 草稿。
- 只有已发布版本才能分配给产线。
- 每条填报保存 version_id，因此历史记录永远按提交时的 Schema 解释。
- 填报时保存班次名称、开始和结束时间快照，避免后续修改班次影响历史。
- 产品名称以字段值形式保存，产品基础数据删除后历史记录仍可读。

## 9. 核心业务流程

### 9.1 登录和会话

~~~mermaid
sequenceDiagram
    participant U as 用户
    participant F as PC/PDA 前端
    participant A as API
    participant D as PostgreSQL
    U->>F: 输入账号或工号
    F->>A: POST /auth/pc/login 或 /auth/pda/login
    A->>D: 查询用户、角色、部门
    A->>A: 校验启用状态、客户端权限、部门状态
    A->>A: PC 登录校验 bcrypt 密码
    A-->>F: JWT + HttpOnly Cookie
    F->>A: GET /me
    A-->>F: 用户、权限、产线、部门、品牌
~~~

### 9.2 表单模板和产线分配

1. 管理员创建模板，系统同时创建 v1 草稿。
2. 管理员设计字段并保存；已有模板保存时创建新草稿版本。
3. 发布草稿后版本状态变为 published，可选地将版本分配给产线。
4. 产线分配表保存当前 active_version_id。
5. 父产线分配可级联到子产线，但子产线的显式版本优先。
6. 回滚只修改产线的活动版本，不删除后续版本。
7. 未使用模板可以删除；已使用模板由清理逻辑保护。

### 9.3 首末检填报

1. PDA/PC 选择产线，后端返回该产线分配的模板和发布版本。
2. 前端获取当前班次和业务日期。
3. 如果是首检，先检查同产线、同模板、同业务日期、同班次是否已有首检记录。
4. 重复首检必须填写原因。
5. 提交时创建主记录、明细行和字段值；所有写入在一个数据库事务中完成。
6. 数字字段超出上下限不阻止提交，但日志和历史页面会标注异常。
7. 同一业务上下文下较新的首末检记录会标记较早记录为已被覆盖。

### 9.4 班次和业务日期

- 时区固定为 Asia/Shanghai。
- 班次计划按 effective_at 小于等于当前时间选择最新计划。
- 普通班次按当天时间判断。
- 跨零点班次以班次开始日期作为业务日期。
- 例如 22:00 至次日 06:00 的班次，在次日 01:00 填报时仍归入前一天业务日期。

### 9.5 废除和恢复

- 主填报可批量废除其所有有效明细，也可按明细单独废除。
- 废除和恢复必须填写原因，原因最多 500 字。
- 废除会保存操作人、时间、原因。
- 恢复会清空废除信息并保存恢复原因、恢复人和时间。
- 所有操作写入 audit_logs，前端历史页可显示状态和原因。
- 权限分为本人权限和授权产线范围权限。

### 9.6 日志、日历和 Excel 导出

- 日志按产线、模板、日期范围、填报人筛选。
- 日历按业务日期汇总有有效明细的填报数量。
- 日志详情会解析提交版本 Schema，显示字段标签、单位、上下限异常和重复值。
- Excel 导出限制单次日期范围不超过 31 天。
- 首检/末检按业务日期分工作表，报表和登记表写入数据工作表。
- 导出时只导出有效明细，首末检会保留同一业务上下文中的最新记录，避免被覆盖记录重复导出。

### 9.7 备份和恢复

| 类型 | 生成者 | 内容 | 使用方式 |
|---|---|---|---|
| 完整备份 | 超级管理员 | PostgreSQL pg_dump --format=custom .dump | 由数据库管理员在服务器侧恢复 |
| 部门归档 | PC 用户按范围 | 部门配置、用户、产线、产品、班次、模板、填报、审计等 ZIP | 超级管理员在备份页上传 ZIP 恢复 |

部门恢复流程：

1. 校验 ZIP 结构、清单和版本。
2. 解析目标部门编码。
3. 在数据库事务中清理并重建该部门业务数据。
4. 重建用户、产线、产品、班次、模板、版本、填报、字段值、审计和仪表报文。
5. 任一环节失败则整体回滚。
6. 恢复结果写入审计记录。

## 10. 权限设计

内置角色：

| 角色 | PC | PDA | 主要能力 |
|---|---|---|---|
| super_admin | 是 | 是 | 全部权限、部门管理、完整备份、部门 ZIP 恢复 |
| admin | 是 | 是 | 本部门管理、表单、日志、部门归档 |
| normal | 否 | 是 | PDA 填报、本人历史、本人废除和恢复 |

权限码分为以下类别：

- 登录：pc.login、pda.login
- 用户与权限：user.manage、role.manage
- 基础配置：line.manage、product.manage、shift.manage
- 表单：form.view、form.manage、form.publish、form.rollback
- 执行：execution.view、execution.fill
- 填报修改：submission.edit_own、submission.edit_scope、submission.void_own、submission.void_scope
- 日志与导出：log.view、export、audit.view
- 系统：backup.manage、system.manage、department.manage

## 11. 接口概览

### 11.1 公共与账户

- POST /api/v1/auth/pc/login
- POST /api/v1/auth/pda/login
- POST /api/v1/auth/logout
- GET /api/v1/me
- GET /health

### 11.2 PC 管理

- 部门：GET/POST/PATCH/DELETE /api/v1/admin/departments
- 用户：GET/POST/PATCH /api/v1/admin/users、密码重置、导入模板与导入接口
- 角色：GET/POST/PATCH /api/v1/admin/roles
- 产线：GET/POST/PATCH/DELETE /api/v1/admin/lines、导入接口
- 产品：GET/POST/PATCH/DELETE /api/v1/admin/products
- 班次：GET/POST/PUT/DELETE /api/v1/admin/shifts
- 模板：GET/POST /api/v1/admin/form-templates、版本创建、发布、回滚、复制、同步、分配
- 日志：GET /api/v1/admin/logs、GET /api/v1/admin/logs/calendar
- 填报状态：POST /api/v1/admin/submissions/:id/void 和 restore
- 导出：POST /api/v1/admin/exports
- 备份：GET/POST /api/v1/admin/backups、下载、恢复
- 审计与仪表：GET /api/v1/admin/audit-logs、GET /api/v1/admin/instrument-messages

### 11.3 执行与 PDA

- GET /api/v1/execution/lines
- GET /api/v1/execution/shifts
- GET /api/v1/execution/forms
- GET /api/v1/execution/forms/:id
- GET /api/v1/execution/products
- GET /api/v1/execution/repeat-check
- GET /api/v1/execution/submissions/current
- GET /api/v1/execution/submissions
- POST /api/v1/execution/submissions
- PATCH /api/v1/execution/rows/:id
- POST /api/v1/execution/rows/:id/void 和 restore
- GET /api/v1/execution/my-submissions
- GET /api/v1/execution/my-submissions/:id

## 12. 安全设计

1. 密码使用 bcrypt cost 12 存储，接口不返回密码散列。
2. JWT 使用 HS256，密钥只从环境变量读取。
3. PC 和 PDA 登录权限分离，PC 管理接口额外要求 PC 客户端 Token。
4. 所有关键数据查询和写入均在后端叠加部门范围过滤，不能只依赖前端隐藏按钮。
5. 超级管理员账号通过数据库触发器、服务层校验和 UI 限制，保证只有一个启用的超级管理员。
6. 废除、恢复、发布、回滚、备份和恢复等关键动作写入审计日志。
7. 导入文件限制大小和扩展名，Excel 解析失败不会写入部分数据。
8. 部门 ZIP 恢复在事务中执行，失败整体回滚。

需要注意的限制：

- PDA 登录按当前设计仅需工号，适合受控内网，不适合直接暴露到公网。
- PC 记住密码会把账号密码保存在浏览器 localStorage，公用电脑不应启用。
- CORS 当前回显请求 Origin，生产环境建议通过反向代理限制来源。
- 完整数据库 .dump 不能通过本系统页面上传恢复，应由数据库管理员使用 PostgreSQL 工具恢复。

## 13. 一致性和事务

系统在以下关键路径使用数据库事务：

- 创建模板和 v1 草稿。
- 发布版本并同步产线活动版本。
- 创建填报主记录、明细行和字段值。
- 保存明细时先删旧字段值再写新字段值。
- 修改班次时创建替代计划并软删除原计划。
- 批量导入产线。
- 恢复部门 ZIP 归档。
- 初始化权限、角色、部门和管理员。

一致性策略：

- 历史记录依赖版本 ID、班次快照和用户信息快照，避免配置修改影响历史。
- 产线、模板、产品等对象使用软删除，避免破坏历史外键。
- 部门编码和业务编码通过部分唯一索引防止并发重复。
- 超级管理员唯一性同时由数据库触发器和应用层校验保证。

## 14. 构建、部署与运维

### 14.1 首次部署

1. 安装 PostgreSQL 18，并创建业务数据库。
2. 复制 server/.env.example 为 server/.env。
3. 修改数据库连接、JWT 密钥、pg_dump 路径和备份目录。
4. 执行迁移：go run ./cmd/migrate up。
5. 执行初始化：go run ./cmd/seed。
6. 构建前端和后端：powershell -ExecutionPolicy Bypass -File scripts\\build.ps1。
7. 启动 API，访问 /pc/ 或 /pda/。

### 14.2 日常运维

- 每日检查 API 进程、数据库连接和磁盘空间。
- 至少每日生成一次完整备份，并将 .dump 复制到异机。
- 每周抽查部门 ZIP 归档和导出 Excel。
- 定期检查 audit_logs、instrument_messages、backup_records 增长。
- 修改系统前先做数据库备份。
- 升级代码后先备份，再执行迁移、构建和回归验证。

## 15. 扩展点

1. 仪表接入：实现 instrument.Ingestor，接入 MQTT 后将原始报文写入 instrument_messages。
2. 填报自动化：将仪表报文解析为 FieldValue，默认标记为 instrument 来源。
3. 报表分析：可在日志和导出服务上增加统计接口或独立报表。
4. 数据生命周期：可为审计、报文和备份记录增加保留策略。
5. 组织扩展：产线目前最多两级，如需更深层级可扩展 lines 和产线展开逻辑。
6. 单点登录：可替换 auth.Service，同时保持现有 user_id 上下文。

## 16. 已知限制和后续建议

- V1 未接入 MQTT，仪表数据仅预留表和接口。
- PDA 登录无密码，建议只在工厂内网开放。
- 恢复仅支持本系统部门 ZIP，完整备份恢复需要 DBA 操作。
- 没有独立的操作手册和架构文档版本管理，建议后续纳入发布流程。
- 建议增加数据库连接池、备份文件校验、自动清理和监控告警配置。
- 建议生产环境使用 Nginx/Ingress 终止 TLS，并限制 CORS、访问来源和上传大小。
- 建议对关键 API 增加审计查询页面和更细粒度的数据权限。

## 17. 结论

当前系统采用前后端分层、部门隔离、角色权限、模板版本化和事务写入的设计，能够支持首末检、报表、登记表和现场追溯等核心场景。代码边界清晰，后续可在不改动主业务模型的前提下扩展仪表采集、统计分析和更细粒度权限。
