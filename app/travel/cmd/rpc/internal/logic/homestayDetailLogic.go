package logic

import (

	"context"
	"encoding/json"
	"fmt"


	"looklook/app/travel/cmd/rpc/internal/svc"
	"looklook/app/travel/cmd/rpc/pb"
	"looklook/app/travel/model"
	"looklook/pkg/xerr"

	"github.com/pkg/errors"
	"github.com/zeromicro/go-zero/core/logx"
)

type HomestayDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewHomestayDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HomestayDetailLogic {
	return &HomestayDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// HomestayDetail homestay detail .
// func (l *HomestayDetailLogic) HomestayDetail(in *pb.HomestayDetailReq) (*pb.HomestayDetailResp, error) {

// 	homestay, err := l.svcCtx.HomestayModel.FindOne(l.ctx, in.Id)
// 	if err != nil && err != model.ErrNotFound {
// 		return nil, errors.Wrapf(xerr.NewErrCode(xerr.DB_ERROR), " HomestayDetail db err , id : %d ", in.Id)
// 	}

// 	var pbHomestay pb.Homestay
// 	if homestay != nil {
// 		_ = copier.Copy(&pbHomestay, homestay)
// 	}

// 	return &pb.HomestayDetailResp{
// 		Homestay: &pbHomestay,
// 	}, nil

// }
func (l *HomestayDetailLogic) HomestayDetail(in *pb.HomestayDetailReq) (*pb.HomestayDetailResp, error) {
	// 1. 构建 L1 缓存的 Key
	cacheKey := fmt.Sprintf("homestay:detail:%d", in.Id)

	// 2. 尝试从 L1 (本地内存) 获取数据
	if val, err := l.svcCtx.LocalCache.Get([]byte(cacheKey)); err == nil {
		var resp pb.HomestayDetailResp
		// 命中 L1，反序列化后直接返回
		if err := json.Unmarshal(val, &resp); err == nil {
			return &resp, nil
		}
	}

	// 3. L1 未命中，使用 singleflight 合并并发请求，防止缓存击穿
	// Do 方法保证同一个 cacheKey 在同一瞬间只会有一个 Goroutine 执行匿名函数中的逻辑
	v, err, _ := l.svcCtx.SingleGroup.Do(cacheKey, func() (interface{}, error) {
		
		// 4. 调用 go-zero 生成的 Model 层
		// 此时底层会走 L2 缓存 (Redis) -> 若未命中再查 MySQL，并回填 Redis
		homestay, err := l.svcCtx.HomestayModel.FindOne(l.ctx, in.Id)
		if err != nil && err != model.ErrNotFound {
			return nil, errors.Wrapf(xerr.NewErrCode(xerr.DB_ERROR), "HomestayDetail Database Exception id:%d, err:%v", in.Id, err)
		}

		// 检查数据是否存在
		if homestay == nil {
			return nil, errors.Wrapf(xerr.NewErrMsg("民宿不存在"), "id:%d", in.Id)
		}

		// 5. 拼装返回结果
		resp := &pb.HomestayDetailResp{
			Homestay: &pb.Homestay{
				Id:               homestay.Id,
				Title:            homestay.Title,
				SubTitle:         homestay.SubTitle,
				Banner:           homestay.Banner,
				Info:             homestay.Info,
				PeopleNum:        homestay.PeopleNum,
				HomestayBusinessId: homestay.HomestayBusinessId,
				UserId:           homestay.UserId,
				RowState:         homestay.RowState,
			},
		}

		// 6. 将结果序列化后回填到 L1 缓存 (LocalCache)
		if respBytes, err := json.Marshal(resp); err == nil {
			_ = l.svcCtx.LocalCache.Set([]byte(cacheKey), respBytes, l.svcCtx.Config.LocalCache.Expire)
		}

		return resp, nil
	})

	if err != nil {
		return nil, err
	}

	// 将 singleflight 共享的结果断言为实际返回类型
	return v.(*pb.HomestayDetailResp), nil
}