package geo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// ============ 门店档案用例（本地生活 GEO 地基）============

// StoreLocationUseCase 编排门店档案管理。
//
// 设计动机：
//   - 地理编码（地址→经纬度）是外部细节（地图供应商），经 port.GeoLocator 注入，
//     用例层只关心业务规则：编码成功回填坐标、失败标记 pending 不阻断创建。
//   - 未配置地图服务（ErrGeoNotConfigured）时门店照常创建（地址先落库），
//     配置 AMAP_API_KEY 后可对 pending 门店批量重试——"先有数据，再补坐标"。
type StoreLocationUseCase struct {
	repo      port.StoreLocationRepository
	brandRepo port.BrandRepository
	locator   port.GeoLocator // 可选；nil=不尝试编码（全部 pending）
}

func NewStoreLocationUseCase(repo port.StoreLocationRepository, brandRepo port.BrandRepository) *StoreLocationUseCase {
	return &StoreLocationUseCase{repo: repo, brandRepo: brandRepo}
}

// SetLocator 注入地理编码服务（可选；未注入时门店保存为 pending）。
func (uc *StoreLocationUseCase) SetLocator(l port.GeoLocator) {
	if l != nil {
		uc.locator = l
	}
}

// StoreLocationInput 创建/更新门店的输入（handler 层绑定 JSON 后转换）。
type StoreLocationInput struct {
	TenantID   string
	BrandID    string
	Name       string
	Address    string // 必填（地理编码的输入）
	Phone      string
	Hours      string
	PriceLevel string
	BizType    string // 业态（LocalBusiness/Restaurant/Cafe/Bar/Store；空=LocalBusiness）
}

// Create 创建门店：先落库（含地址），再尝试地理编码回填（失败标记 pending）。
// 返回门店 + 编码是否成功（供 handler 提示"地址未解析成功，稍后可重试"）。
func (uc *StoreLocationUseCase) Create(ctx context.Context, in StoreLocationInput) (entity.StoreLocation, error) {
	if in.Address == "" {
		return entity.StoreLocation{}, fmt.Errorf("门店地址不能为空")
	}
	// 租户校验：品牌必须属于当前租户（防越权挂到他人品牌下）
	if _, err := uc.brandRepo.FindByID(ctx, in.TenantID, in.BrandID); err != nil {
		return entity.StoreLocation{}, fmt.Errorf("品牌不存在: %w", err)
	}
	now := time.Now()
	loc := entity.StoreLocation{
		ID:         fmt.Sprintf("sl-%d", now.UnixNano()),
		TenantID:   in.TenantID,
		BrandID:    in.BrandID,
		Name:       in.Name,
		Address:    in.Address,
		Phone:      in.Phone,
		Hours:      in.Hours,
		PriceLevel: in.PriceLevel,
		BizType:    in.BizType,
		GeoStatus:  entity.GeoStatusPending,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if loc.BizType == "" {
		loc.BizType = "LocalBusiness" // 默认业态（schema.org 通用父类）
	}
	uc.applyGeocode(ctx, &loc)
	if err := uc.repo.Save(ctx, loc); err != nil {
		return entity.StoreLocation{}, fmt.Errorf("save store: %w", err)
	}
	return loc, nil
}

// Update 更新门店：重新触发地理编码（地址变更后坐标必须重算）。
func (uc *StoreLocationUseCase) Update(ctx context.Context, tenantID, brandID, id string, in StoreLocationInput) (entity.StoreLocation, error) {
	loc, err := uc.repo.FindByID(ctx, tenantID, id)
	if err != nil {
		return entity.StoreLocation{}, err
	}
	// 防越权：门店必须挂在同品牌下
	if loc.BrandID != brandID {
		return entity.StoreLocation{}, errors.New("门店不属于该品牌")
	}
	loc.Name = in.Name
	loc.Phone = in.Phone
	loc.Hours = in.Hours
	loc.PriceLevel = in.PriceLevel
	if in.BizType != "" {
		loc.BizType = in.BizType
	}
	if in.Address != "" {
		loc.Address = in.Address
		loc.GeoStatus = entity.GeoStatusPending // 地址变了，旧坐标作废
		uc.applyGeocode(ctx, &loc)
	}
	loc.UpdatedAt = time.Now()
	if err := uc.repo.Save(ctx, loc); err != nil {
		return entity.StoreLocation{}, fmt.Errorf("save store: %w", err)
	}
	return loc, nil
}

// ReGeocode 重试地理编码（pending/failed 门店：配置地图服务后一键补齐坐标）。
func (uc *StoreLocationUseCase) ReGeocode(ctx context.Context, tenantID, id string) (entity.StoreLocation, error) {
	loc, err := uc.repo.FindByID(ctx, tenantID, id)
	if err != nil {
		return entity.StoreLocation{}, err
	}
	uc.applyGeocode(ctx, &loc)
	loc.UpdatedAt = time.Now()
	if err := uc.repo.Save(ctx, loc); err != nil {
		return entity.StoreLocation{}, fmt.Errorf("save store: %w", err)
	}
	return loc, nil
}

// List 列出品牌的门店。
func (uc *StoreLocationUseCase) List(ctx context.Context, tenantID, brandID string) ([]entity.StoreLocation, error) {
	return uc.repo.ListByBrand(ctx, tenantID, brandID)
}

// Delete 删除门店（先查再删，租户隔离）。
func (uc *StoreLocationUseCase) Delete(ctx context.Context, tenantID, brandID, id string) error {
	loc, err := uc.repo.FindByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if loc.BrandID != brandID {
		return errors.New("门店不属于该品牌")
	}
	return uc.repo.Delete(ctx, tenantID, id)
}

// applyGeocode 尝试地理编码并回填（内部方法）：
//   - 未注入 locator / 未配置地图服务 → 保持 pending（不阻断）
//   - 编码成功 → 回填坐标+区划，再逆编码补商圈，标记 ok
//   - 编码失败（地址无法解析）→ 标记 failed
func (uc *StoreLocationUseCase) applyGeocode(ctx context.Context, loc *entity.StoreLocation) {
	if uc.locator == nil {
		return
	}
	res, err := uc.locator.Geocode(ctx, loc.Address)
	if err != nil {
		if errors.Is(err, port.ErrGeoNotConfigured) {
			loc.GeoStatus = entity.GeoStatusPending
			return
		}
		loc.GeoStatus = entity.GeoStatusFailed
		return
	}
	loc.Lat = res.Lat
	loc.Lng = res.Lng
	loc.City = res.City
	loc.District = res.District
	loc.Adcode = res.Adcode
	loc.GeoStatus = entity.GeoStatusOK
	// 商圈补全（P1）：坐标就绪后逆编码拿商圈（如"望京"）——失败不阻断（商圈是增强项）
	if regeo, rErr := uc.locator.ReverseGeocode(ctx, res.Lng, res.Lat); rErr == nil && regeo.BusinessArea != "" {
		loc.BusinessArea = regeo.BusinessArea
	}
}
