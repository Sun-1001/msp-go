# Go 数据库迁移策略

`backend/migrations/` 是当前数据库结构的唯一迁移来源。`0001_initial_schema.up.sql` 建立完整初始结构，后续变更按版本顺序追加 forward migration。

## 新增迁移

- 文件名使用 `NNNN_description.up.sql`，版本号连续且一经发布不可复用。
- 不修改 `0001_initial_schema.up.sql` 承载增量变化，也不重建已经发布的迁移历史。
- 每个迁移保持单一目的，明确约束、索引、默认值和数据修正的执行顺序。
- 结构或数据变更必须经过评审；禁止在没有迁移记录的情况下直接修改共享数据库。

## 执行与验证

在 `backend/` 目录运行：

```powershell
go run ./cmd/migrate
go run ./cmd/migrate
```

首次执行记录实际 `applied_count`，紧接着重复执行应为 `applied_count=0`。发布前还应在代表性数据库或一次性空库验证关键表、约束、索引、默认值和必要的数据校准结果。

## 发布与回滚

- 生产迁移前先创建并验证可恢复备份，迁移成功后再启动依赖新结构的 API 版本。
- migration runner 不由 API 自动执行，部署流程必须显式运行。
- 不提供自动 down migration；失败时恢复备份，或发布经过评审的补偿性 forward migration。
- 回滚应用镜像前，先确认旧版本能够读取迁移后的结构。
- 仓库外数据库若执行过未发布或不同内容的同版本迁移，先核对实际 schema 和 `go_schema_migrations`，不得通过删除版本记录盲目重放。

历史 Alembic 链已退出当前工作区；需要追溯时使用 Git 历史或已归档的发布材料。
