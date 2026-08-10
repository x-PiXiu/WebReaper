package repository

import (
	"webreaper/internal/domain/entity"
)

// ---- GEO mapper（实体 ↔ PO 双向转换）----

func brandToPO(e entity.Brand) BrandPO {
	return BrandPO{
		ID: e.ID, TenantID: e.TenantID, Name: e.Name, Positioning: e.Positioning,
		CoreSelling: toJSON(e.CoreSelling), Competitors: toJSON(e.Competitors),
		CreatedAt: e.CreatedAt,
	}
}

func brandFromPO(p BrandPO) entity.Brand {
	return entity.Brand{
		ID: p.ID, TenantID: p.TenantID, Name: p.Name, Positioning: p.Positioning,
		CoreSelling: toStringSlice(p.CoreSelling), Competitors: toStringSlice(p.Competitors),
		CreatedAt: p.CreatedAt,
	}
}

func keywordToPO(e entity.Keyword) KeywordPO {
	return KeywordPO{ID: e.ID, TenantID: e.TenantID, BrandID: e.BrandID, Term: e.Term, Intent: e.Intent, CreatedAt: e.CreatedAt}
}

func keywordFromPO(p KeywordPO) entity.Keyword {
	return entity.Keyword{ID: p.ID, TenantID: p.TenantID, BrandID: p.BrandID, Term: p.Term, Intent: p.Intent, CreatedAt: p.CreatedAt}
}

func monitoringResultToPO(e entity.MonitoringResult) MonitoringResultPO {
	return MonitoringResultPO{
		ID: e.ID, TenantID: e.TenantID, BrandID: e.BrandID, KeywordID: e.KeywordID,
		EngineName: e.EngineName, SampleCount: e.SampleCount, MentionCount: e.MentionCount,
		MentionRate: e.MentionRate, AvgPosition: e.AvgPosition, Sentiment: e.Sentiment,
		Competitors: toJSON(e.Competitors), CompetitorRates: toFloatMap(e.CompetitorRates),
		Confidence: e.Confidence, ProbedAt: e.ProbedAt,
		RawSample: e.RawSample,
	}
}

func monitoringResultFromPO(p MonitoringResultPO) entity.MonitoringResult {
	return entity.MonitoringResult{
		ID: p.ID, TenantID: p.TenantID, BrandID: p.BrandID, KeywordID: p.KeywordID,
		EngineName: p.EngineName, SampleCount: p.SampleCount, MentionCount: p.MentionCount,
		MentionRate: p.MentionRate, AvgPosition: p.AvgPosition, Sentiment: p.Sentiment,
		Competitors: toStringSlice(p.Competitors), CompetitorRates: toFloatMapFromJSON(p.CompetitorRates),
		Confidence: p.Confidence, ProbedAt: p.ProbedAt,
		RawSample: p.RawSample,
	}
}

func optimizedContentToPO(e entity.OptimizedContent) OptimizedContentPO {
	return OptimizedContentPO{
		ID: e.ID, Title: e.Title, TenantID: e.TenantID, BrandID: e.BrandID, KeywordID: e.KeywordID,
		OriginalText: e.OriginalText, OptimizedText: e.OptimizedText, Version: e.Version,
		ScoreTotal: e.Score.Total, Authority: e.Score.Authority, Specificity: e.Score.Specificity,
		Structure: e.Score.Structure, Uniqueness: e.Score.Uniqueness, Recency: e.Score.Recency,
		Status: e.Status, CreatedAt: e.CreatedAt,
	}
}

func optimizedContentFromPO(p OptimizedContentPO) entity.OptimizedContent {
	return entity.OptimizedContent{
		ID: p.ID, Title: p.Title, TenantID: p.TenantID, BrandID: p.BrandID, KeywordID: p.KeywordID,
		OriginalText: p.OriginalText, OptimizedText: p.OptimizedText, Version: p.Version,
		Score: entity.GEOScore{
			Total: p.ScoreTotal, Authority: p.Authority, Specificity: p.Specificity,
			Structure: p.Structure, Uniqueness: p.Uniqueness, Recency: p.Recency,
		},
		Status: p.Status, CreatedAt: p.CreatedAt,
	}
}
