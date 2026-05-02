package service

import (
	"errors"
	"stock-ai/internal/db"
	"stock-ai/internal/model"
)

// StrategyService 策略业务逻辑
type StrategyService struct{}

// NewStrategyService 创建策略服务实例
func NewStrategyService() *StrategyService {
	return &StrategyService{}
}

// CreateStrategyReq 创建/更新策略的请求体
type CreateStrategyReq struct {
	Name        string               `json:"name" binding:"required"`
	LogicalOp   string               `json:"logical_op"` // AND / OR (前端传大写)
	Signals     []model.StrategySignal `json:"signals"`
	Description string              `json:"description"`
}

// RenameReq 重命名请求
type RenameReq struct {
	Name string `json:"name" binding:"required"`
}

// BatchDeleteReq 批量删除请求
type BatchDeleteReq struct {
	IDs []uint `json:"ids" binding:"required,min=1"`
}

// normalizeLogicalOp 将前端的 AND/OR 转为内部 LogicalOp 类型
func normalizeLogicalOp(op string) model.LogicalOp {
	switch op {
	case "OR", "or":
		return model.LogicalOR
	default:
		return model.LogicalAND
	}
}

// Create 创建新策略
func (svc *StrategyService) Create(req *CreateStrategyReq, uid uint) (*model.StrategyDetail, error) {
	strategy := &model.Strategy{
		UID:         uid,
		Name:        req.Name,
		LogicalOp:    normalizeLogicalOp(req.LogicalOp),
		Description: req.Description,
	}

	if err := strategy.SetConditions(req.Signals); err != nil {
		return nil, err
	}

	if err := db.CreateStrategy(strategy); err != nil {
		return nil, err
	}

	return strategy.ToDetail()
}

// GetByID 根据 ID 获取策略详情
func (svc *StrategyService) GetByID(id uint) (*model.StrategyDetail, error) {
	s, err := db.GetStrategyByID(id)
	if err != nil {
		return nil, err
	}
	return s.ToDetail()
}

// List 策略列表（分页+搜索）
type ListResp struct {
	List  []model.StrategyDetail `json:"list"`
	Total int64                  `json:"total"`
	Page  int                    `json:"page"`
	Size  int                    `json:"size"`
}

func (svc *StrategyService) List(uid uint, keyword string, page, pageSize int) (*ListResp, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	strategies, total, err := db.ListStrategies(uid, keyword, page, pageSize)
	if err != nil {
		return nil, err
	}

	details := make([]model.StrategyDetail, 0, len(strategies))
	for _, s := range strategies {
		d, err := s.ToDetail()
		if err != nil {
			continue // 单个解析失败不中断，跳过该条
		}
		details = append(details, *d)
	}

	return &ListResp{
		List:  details,
		Total: total,
		Page:  page,
		Size:  pageSize,
	}, nil
}

// Update 更新策略（全量更新，保留原有 UID）
func (svc *StrategyService) Update(id uint, req *CreateStrategyReq) (*model.StrategyDetail, error) {
	s, err := db.GetStrategyByID(id)
	if err != nil {
		return nil, errors.New("策略不存在")
	}

	// 显式保留原始 UID，防止被意外清零
	originalUID := s.UID
	s.Name = req.Name
	s.LogicalOp = normalizeLogicalOp(req.LogicalOp)
	s.Description = req.Description
	s.UID = originalUID // 确保 UID 不变

	if err := s.SetConditions(req.Signals); err != nil {
		return nil, err
	}

	if err := db.UpdateStrategy(s); err != nil {
		return nil, err
	}

	return s.ToDetail()
}

// Rename 重命名策略
func (svc *StrategyService) Rename(id uint, newName string) error {
	return db.RenameStrategy(id, newName)
}

// Delete 删除单个策略
func (svc *StrategyService) Delete(id, uid uint) error {
	return db.DeleteStrategyByID(id, uid)
}

// BatchDelete 批量删除策略
func (svc *StrategyService) BatchDelete(ids []uint, uid uint) error {
	if len(ids) == 0 {
		return errors.New("删除列表不能为空")
	}
	return db.DeleteStrategyByIDs(ids, uid)
}
