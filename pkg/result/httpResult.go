package result

import (
	"fmt"
	"net/http"

	"looklook/pkg/xerr"

	"github.com/pkg/errors"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
	"google.golang.org/grpc/status"
)

// http返回
func HttpResult(r *http.Request, w http.ResponseWriter, resp interface{}, err error) {
	if err == nil {
		// 优化：使用对象池包装成功返回
		successBean := GetSuccessBean(resp)
		// httpx.WriteJson 是同步写入，写入完成后可以直接回收
		defer ReleaseSuccessBean(successBean)

		httpx.WriteJson(w, http.StatusOK, successBean)
	} else {
		errcode := xerr.SERVER_COMMON_ERROR
		errmsg := "服务器开小差啦，稍后再来试一试"

		causeErr := errors.Cause(err)
		if e, ok := causeErr.(*xerr.CodeError); ok {
			errcode = e.GetErrCode()
			errmsg = e.GetErrMsg()
		} else {
			if gstatus, ok := status.FromError(causeErr); ok {
				grpcCode := uint32(gstatus.Code())
				if xerr.IsCodeErr(grpcCode) {
					errcode = grpcCode
					errmsg = gstatus.Message()
				}
			}
		}

		logx.WithContext(r.Context()).Errorf("【API-ERR】 : %+v ", err)

		// 优化：使用对象池包装错误返回
		errorBean := GetErrorBean(errcode, errmsg)
		defer ReleaseErrorBean(errorBean)

		httpx.WriteJson(w, http.StatusBadRequest, errorBean)
	}
}

// 授权的http方法
func AuthHttpResult(r *http.Request, w http.ResponseWriter, resp interface{}, err error) {
	if err == nil {
		successBean := GetSuccessBean(resp)
		defer ReleaseSuccessBean(successBean)

		httpx.WriteJson(w, http.StatusOK, successBean)
	} else {
		errcode := xerr.SERVER_COMMON_ERROR
		errmsg := "服务器开小差啦，稍后再来试一试"

		causeErr := errors.Cause(err)
		if e, ok := causeErr.(*xerr.CodeError); ok {
			errcode = e.GetErrCode()
			errmsg = e.GetErrMsg()
		} else {
			if gstatus, ok := status.FromError(causeErr); ok {
				grpcCode := uint32(gstatus.Code())
				if xerr.IsCodeErr(grpcCode) {
					errcode = grpcCode
					errmsg = gstatus.Message()
				}
			}
		}

		logx.WithContext(r.Context()).Errorf("【GATEWAY-ERR】 : %+v ", err)

		errorBean := GetErrorBean(errcode, errmsg)
		defer ReleaseErrorBean(errorBean)

		httpx.WriteJson(w, http.StatusUnauthorized, errorBean)
	}
}

// http 参数错误返回
func ParamErrorResult(r *http.Request, w http.ResponseWriter, err error) {
	errMsg := fmt.Sprintf("%s ,%s", xerr.MapErrMsg(xerr.REUQEST_PARAM_ERROR), err.Error())

	// 优化：复用池化 ErrorBean
	errorBean := GetErrorBean(xerr.REUQEST_PARAM_ERROR, errMsg)
	defer ReleaseErrorBean(errorBean)

	httpx.WriteJson(w, http.StatusBadRequest, errorBean)
}
