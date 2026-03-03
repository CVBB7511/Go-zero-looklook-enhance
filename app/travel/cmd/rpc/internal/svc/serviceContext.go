package svc

import (
	"github.com/coocood/freecache"
	"golang.org/x/sync/singleflight"
	"looklook/app/travel/cmd/rpc/internal/config"
	"looklook/app/travel/model"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config config.Config

	HomestayModel model.HomestayModel

	// 新增：本地缓存实例与 Singleflight
	LocalCache    *freecache.Cache
	SingleGroup   *singleflight.Group
}

func NewServiceContext(c config.Config) *ServiceContext {
	sqlConn:= sqlx.NewMysql(c.DB.DataSource)
	return &ServiceContext{
		Config: c,
		HomestayModel: model.NewHomestayModel(sqlConn, c.Cache),
		LocalCache:  freecache.NewCache(c.LocalCache.CacheSize),
		SingleGroup: new(singleflight.Group), // 初始化指针
	}
}
