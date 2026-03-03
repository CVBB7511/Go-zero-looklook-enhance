package jobtype

import (
	"looklook/app/order/model"
	"sync"
)

// DeferCloseHomestayOrderPayload defer close homestay order
type DeferCloseHomestayOrderPayload struct {
	Sn string
}

// PaySuccessNotifyUserPayload pay success notify user
type PaySuccessNotifyUserPayload struct {
	Order *model.HomestayOrder
}

// 优化：为高频产生的消费任务 payload 添加对象池
var PaySuccessNotifyUserPayloadPool = sync.Pool{
	New: func() interface{} {
		return &PaySuccessNotifyUserPayload{}
	},
}

// Reset 优化：放回池前清理指针引用，防止内存泄漏 (Memory Leak)
func (p *PaySuccessNotifyUserPayload) Reset() {
	p.Order = nil
}
