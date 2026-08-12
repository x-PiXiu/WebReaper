package repository

import (
	"encoding/json"

	"webreaper/internal/domain/entity"
)

// ---- 发布账号 mapper（实体 ↔ PO 双向转换）----

func accountToPO(e entity.Account) AccountPO {
	return AccountPO{
		ID: e.ID, TenantID: e.TenantID, Platform: e.Platform, DisplayName: e.DisplayName,
		CookieEncrypted: e.CookieEncrypted, Health: e.Health, LoginMethod: e.LoginMethod,
		ExpiresAt: e.ExpiresAt, BoundAt: e.BoundAt, LastUsedAt: e.LastUsedAt,
	}
}

func accountFromPO(p AccountPO) entity.Account {
	health := p.Health
	if health == "" {
		health = entity.AccountHealthActive
	}
	return entity.Account{
		ID: p.ID, TenantID: p.TenantID, Platform: p.Platform, DisplayName: p.DisplayName,
		CookieEncrypted: p.CookieEncrypted, Health: health, LoginMethod: p.LoginMethod,
		ExpiresAt: p.ExpiresAt, BoundAt: p.BoundAt, LastUsedAt: p.LastUsedAt,
	}
}

func publishJobToPO(e entity.PublishJob) PublishJobPO {
	return PublishJobPO{
		ID: e.ID, TenantID: e.TenantID, AccountID: e.AccountID, Platform: e.Platform,
		ContentID: e.ContentID, BrandID: e.BrandID, Title: e.Title, Content: e.Content,
		Mode: e.Mode, Status: e.Status, ExternalURL: e.ExternalURL,
		ErrorMsg: e.ErrorMsg, CreatedAt: e.CreatedAt, PublishedAt: e.PublishedAt,
		PreMentionRate: e.PreMentionRate, PostMentionRate: e.PostMentionRate,
		ScheduledAt: e.ScheduledAt, StoreAddress: e.StoreAddress,
		ContentType: e.ContentType, MediaURLsJSON: mediaURLsToJSON(e.MediaURLs), CoverURL: e.CoverURL,
	}
}

func publishJobFromPO(p PublishJobPO) entity.PublishJob {
	mode := p.Mode
	if mode == "" {
		mode = entity.PublishModeSemiAuto
	}
	status := p.Status
	if status == "" {
		status = entity.PublishStatusPending
	}
	return entity.PublishJob{
		ID: p.ID, TenantID: p.TenantID, AccountID: p.AccountID, Platform: p.Platform,
		ContentID: p.ContentID, BrandID: p.BrandID, Title: p.Title, Content: p.Content,
		Mode: mode, Status: status, ExternalURL: p.ExternalURL,
		ErrorMsg: p.ErrorMsg, CreatedAt: p.CreatedAt, PublishedAt: p.PublishedAt,
		PreMentionRate: p.PreMentionRate, PostMentionRate: p.PostMentionRate,
		ScheduledAt: p.ScheduledAt, StoreAddress: p.StoreAddress,
		ContentType: p.ContentType, MediaURLs: mediaURLsFromJSON(p.MediaURLsJSON), CoverURL: p.CoverURL,
	}
}

// mediaURLsToJSON MediaURLs → JSON 文本（空数组存 "[]"）。
func mediaURLsToJSON(urls []string) string {
	if len(urls) == 0 {
		return "[]"
	}
	b, err := json.Marshal(urls)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// mediaURLsFromJSON JSON 文本 → MediaURLs（容错：空/格式错返回 nil）。
func mediaURLsFromJSON(s string) []string {
	if s == "" {
		return nil
	}
	var urls []string
	if err := json.Unmarshal([]byte(s), &urls); err != nil {
		return nil
	}
	if len(urls) == 0 {
		return nil
	}
	return urls
}
