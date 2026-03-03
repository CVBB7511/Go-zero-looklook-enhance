package result

import "sync"

type ResponseSuccessBean struct {
	Code uint32      `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

type NullJson struct{}

// 优化：全局成功响应对象池
var successBeanPool = sync.Pool{
	New: func() interface{} {
		return &ResponseSuccessBean{Code: 200, Msg: "OK"}
	},
}

func GetSuccessBean(data interface{}) *ResponseSuccessBean {
	bean := successBeanPool.Get().(*ResponseSuccessBean)
	bean.Data = data
	return bean
}

func ReleaseSuccessBean(bean *ResponseSuccessBean) {
	bean.Data = nil // 断开引用，帮助 GC 回收底层实际的 data 对象
	successBeanPool.Put(bean)
}

// 兼容老代码保留，但推荐高并发使用池化方法
func Success(data interface{}) *ResponseSuccessBean {
	return &ResponseSuccessBean{200, "OK", data}
}

type ResponseErrorBean struct {
	Code uint32 `json:"code"`
	Msg  string `json:"msg"`
}

// 优化：全局失败响应对象池
var errorBeanPool = sync.Pool{
	New: func() interface{} {
		return &ResponseErrorBean{}
	},
}

func GetErrorBean(errCode uint32, errMsg string) *ResponseErrorBean {
	bean := errorBeanPool.Get().(*ResponseErrorBean)
	bean.Code = errCode
	bean.Msg = errMsg
	return bean
}

func ReleaseErrorBean(bean *ResponseErrorBean) {
	bean.Msg = "" // 清除字符串，避免潜在的内存堆积
	errorBeanPool.Put(bean)
}

func Error(errCode uint32, errMsg string) *ResponseErrorBean {
	return &ResponseErrorBean{errCode, errMsg}
}