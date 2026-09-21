package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	wlog "github.com/wueasy/wueasy-go-tools/log"
	"github.com/wueasy/wueasy-go-tools/result"

	"waymark/internal/auth"
)

// 业务返回码。
const (
	codeFail         = 1001
	codeConflict     = 1002
	codeUnauthorized = 401
	codeForbidden    = 403
)

// ok 返回成功结果。
func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, result.Ok(data))
}

// okNull 返回无数据的成功结果。
func okNull(c *gin.Context) {
	c.JSON(http.StatusOK, result.OkNull())
}

// fail 返回业务错误，并在服务端记录失败原因。
func fail(c *gin.Context, msg string) {
	logFail(c, msg)
	c.JSON(http.StatusOK, result.Fail(codeFail, msg))
}

// failCode 返回指定错误码的业务错误。
func failCode(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, result.Fail(code, msg))
}

// unauthorized 返回未认证错误。
func unauthorized(c *gin.Context, msg string) {
	logFail(c, msg)
	c.JSON(http.StatusUnauthorized, result.Fail(codeUnauthorized, msg))
}

// forbidden 返回无权限错误。
func forbidden(c *gin.Context, msg string) {
	logFail(c, msg)
	c.JSON(http.StatusForbidden, result.Fail(codeForbidden, msg))
}

// requestUser 返回当前登录用户名，未登录时返回 "-"。
func requestUser(c *gin.Context) string {
	if p := auth.CurrentPrincipal(c); p != nil {
		return p.User.Username
	}
	return "-"
}

// logFail 记录请求处理失败，便于服务端排查（请求ID由 wlog 依据上下文自动附加）。
// 日志带 "[fail]" 前缀便于检索，并附上脱敏后的查询参数（如实例心跳的实例标识）。
func logFail(c *gin.Context, msg string) {
	query := wlog.DesensitizeQuery(c.Request.URL.RawQuery)
	if query != "" {
		query = " query=" + query
	}
	wlog.Ctx(c.Request.Context()).Warnf("[fail] user=%s ip=%s %s %s%s msg=%s",
		requestUser(c), c.ClientIP(), c.Request.Method, c.Request.URL.Path, query, msg)
}

// auditf 记录关键业务操作审计日志。
// 注意：user/ip 作为格式化参数而非拼接到 format 中，避免用户名含 % 导致日志格式错乱。
func auditf(c *gin.Context, format string, args ...any) {
	wlog.Ctx(c.Request.Context()).Infof("[audit] user=%s ip=%s "+format,
		append([]any{requestUser(c), c.ClientIP()}, args...)...)
}
