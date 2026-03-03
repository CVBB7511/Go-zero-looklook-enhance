package thirdPayment

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sync"

	"looklook/app/payment/cmd/api/internal/logic/thirdPayment"
	"looklook/app/payment/cmd/api/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

var bufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 1024))
	},
}

func ThirdPaymentWxPayCallbackHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		buf := bufferPool.Get().(*bytes.Buffer)
		buf.Reset()
		defer bufferPool.Put(buf)

		if r.Body != nil {
			_, err := io.Copy(buf, r.Body)
			if err == nil {
				r.Body.Close()
				r.Body = io.NopCloser(buf)
			}
		}

		l := thirdPayment.NewThirdPaymentWxPayCallbackLogic(r.Context(), svcCtx)
		resp, err := l.ThirdPaymentWxPayCallback(w, r)

		if err != nil {
			logx.WithContext(r.Context()).Errorf("【API-ERR】 ThirdPaymentWxPayCallbackHandler : %+v ", err)
			w.WriteHeader(http.StatusBadRequest)
			return // 【核心修复】: 如果出错（比如验签失败），必须立刻 return，不能往下走
		}

		w.WriteHeader(http.StatusOK)
		// 【核心修复】: 增加判空，防止业务层未返回 resp 导致空指针崩溃
		if resp != nil {
			logx.Infof("ReturnCode : %s ", resp.ReturnCode)
			fmt.Fprint(w, resp.ReturnCode)
		}
	}
}
