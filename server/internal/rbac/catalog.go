// 文件说明：集中定义权限码和权限目录，供初始化与授权中间件使用。

package rbac

const (
	PermPCLogin             = "pc.login"
	PermPDALogin            = "pda.login"
	PermUserManage          = "user.manage"
	PermRoleManage          = "role.manage"
	PermLineManage          = "line.manage"
	PermProductManage       = "product.manage"
	PermShiftManage         = "shift.manage"
	PermFormView            = "form.view"
	PermFormManage          = "form.manage"
	PermFormPublish         = "form.publish"
	PermFormRollback        = "form.rollback"
	PermExecutionView       = "execution.view"
	PermExecutionFill       = "execution.fill"
	PermSubmissionEditOwn   = "submission.edit_own"
	PermSubmissionEditScope = "submission.edit_scope"
	PermSubmissionVoidOwn   = "submission.void_own"
	PermSubmissionVoidScope = "submission.void_scope"
	PermLogView             = "log.view"
	PermExport              = "export"
	PermAuditView           = "audit.view"
	PermBackupManage        = "backup.manage"
	PermSystemManage        = "system.manage"
	PermDepartmentManage    = "department.manage"
)

type PermissionDefinition struct {
	Code  string
	Name  string
	Group string
}

var Catalog = []PermissionDefinition{
	{PermPCLogin, "允许登录PC", "登录"},
	{PermPDALogin, "允许登录PDA", "登录"},
	{PermUserManage, "管理普通用户", "用户与权限"},
	{PermRoleManage, "管理角色权限", "用户与权限"},
	{PermLineManage, "管理产线", "基础配置"},
	{PermProductManage, "维护产品", "基础配置"},
	{PermShiftManage, "管理班次", "基础配置"},
	{PermFormView, "查看表单模板", "表单"},
	{PermFormManage, "维护表单模板", "表单"},
	{PermFormPublish, "发布表单版本", "表单"},
	{PermFormRollback, "回滚表单版本", "表单"},
	{PermExecutionView, "查看填报页面", "执行"},
	{PermExecutionFill, "填报任务和表单", "执行"},
	{PermSubmissionEditOwn, "修改自己的填报", "填报"},
	{PermSubmissionEditScope, "修改授权产线填报", "填报"},
	{PermSubmissionVoidOwn, "作废自己的明细", "填报"},
	{PermSubmissionVoidScope, "作废授权产线明细", "填报"},
	{PermLogView, "查看业务日志", "日志"},
	{PermExport, "导出Excel", "日志"},
	{PermAuditView, "查看审计历史", "审计"},
	{PermBackupManage, "数据库备份", "系统"},
	{PermSystemManage, "系统设置", "系统"},
	{PermDepartmentManage, "维护部门", "系统"},
}

func AllPermissionCodes() []string {
	out := make([]string, 0, len(Catalog))
	for _, p := range Catalog {
		out = append(out, p.Code)
	}
	return out
}
