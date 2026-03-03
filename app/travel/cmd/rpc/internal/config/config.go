package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	DB struct {
		DataSource string
	}
	Cache cache.CacheConf

	// 添加 json tag，保持与 yaml 中的字段匹配
	LocalCache struct {
		CacheSize int `json:"CacheSize,default=104857600"`
		Expire    int `json:"Expire,default=60"`
	} `json:"LocalCache"`
}
