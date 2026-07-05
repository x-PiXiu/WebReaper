package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

// handleSearch 知识语义搜索
func (r *Router) handleSearch(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		fail(c, fmt.Errorf("query param 'q' is required"))
		return
	}
	if r.knowledgeSearch == nil {
		success(c, []any{})
		return
	}
	results, err := r.knowledgeSearch.SearchKnowledge(c.Request.Context(), query, 5)
	if err != nil {
		fail(c, err)
		return
	}
	views := make([]gin.H, 0, len(results))
	for _, r := range results {
		views = append(views, gin.H{
			"id":       r.ID,
			"score":    r.Score,
			"metadata": r.Metadata,
		})
	}
	success(c, views)
}
