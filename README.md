# 轴承首末检系统 V1

这是一个局域网内使用的首末检、报表和登记表系统。

## 技术栈
- Go 1.27、Gin、GORM
- PostgreSQL 18
- Vue 3、TypeScript、Vite
- PC 端 Element Plus，PDA 端 Vant
- 自研 RBAC、JWT、Excelize、pg_dump

## 本地启动
1. 复制 `server/.env.example` 为 `server/.env`，填写本机 PostgreSQL 信息。
2. 在 `server` 目录执行 `go run ./cmd/migrate up`。
3. 在 `server` 目录执行 `go run ./cmd/seed`。
4. 在 `server` 目录执行 `go run ./cmd/api`。
5. 访问 PC 端：`http://127.0.0.1:8080/pc/`。
6. 访问 PDA 端：`http://127.0.0.1:8080/pda/`。
7. 默认管理员：账号 `admin`，密码 `1234`。

也可以在根目录执行：
- `powershell -ExecutionPolicy Bypass -File scripts\migrate.ps1 up`
- `powershell -ExecutionPolicy Bypass -File scripts\build.ps1`
- `powershell -ExecutionPolicy Bypass -File scripts\dev.ps1`

开发前端时，可在根目录执行 `npm.cmd install`，然后分别运行 `npm.cmd run dev:pc` 和 `npm.cmd run dev:pda`。

## 注意
- 数据库密码、JWT 密钥只放在未跟踪的 `.env` 中。
- `pg_dump` 路径通过 `PG_DUMP_PATH` 配置。
- V1 未启用 MQTT，但已创建 `instrument_messages` 表和 `InstrumentIngestor` 接口。
- 多部门版本会将现有数据迁移到 `donggang / 东港分厂`，超级管理员保持全局账号且不绑定部门。
- 系统只允许一个启用的超级管理员；接口、角色选项、批量导入和数据库触发器均禁止创建或分配第二个超级管理员。
- 普通管理员只能查看和维护本部门数据；完整数据库备份仅超级管理员可生成和下载，普通管理员生成部门 ZIP 归档。
- 超级管理员可在备份页上传部门 ZIP 归档进行恢复；恢复按归档中的部门编码覆盖该部门业务数据，并使用数据库事务保证失败回滚。
- 超级管理员在 PC 端通过顶栏切换部门；选择“全部部门”时仅用于跨部门查看，写操作需先选择具体部门。
## GoLand 一键开发启动

在 GoLand 中打开 D:\\code\\DG\\server，运行 server\\cmd\\dev\\main.go，会自动启动数据库迁移、初始化、API、PC 前端和 PDA 前端。按 Ctrl+C 停止全部服务。
