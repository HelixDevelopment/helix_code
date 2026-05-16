package repomap

import (
	"context"
	"encoding/json"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/tools"
)

type RepoMapTool struct {
	repoMap *RepoMap
	approval.DefaultLevelEdit
}

func NewRepoMapTool(repoMap *RepoMap) *RepoMapTool {
	return &RepoMapTool{repoMap: repoMap}
}

func (t *RepoMapTool) Name() string        { return "repomap" }
func (t *RepoMapTool) Description() string { return "Generate a semantic map of the codebase using tree-sitter" }
func (t *RepoMapTool) Category() tools.ToolCategory {
	return tools.ToolCategory("mapping")
}
func (t *RepoMapTool) RequiresApproval() approval.ApprovalLevel { return approval.LevelReadOnly }

func (t *RepoMapTool) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Type:       "object",
		Properties: map[string]interface{}{},
		Required:   []string{},
	}
}

func (t *RepoMapTool) Validate(params map[string]interface{}) error { return nil }

func (t *RepoMapTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	stats, err := t.repoMap.GetStatistics()
	if err != nil {
		return nil, err
	}

	ctx2, err := t.repoMap.GetOptimalContext("", nil)
	if err != nil {
		return nil, err
	}

	type fileSummary struct {
		Path       string  `json:"path"`
		Symbols    int     `json:"symbols"`
		Relevance  float64 `json:"relevance"`
		TokenCount int     `json:"token_count"`
	}
	var summaries []fileSummary
	for _, f := range ctx2 {
		summaries = append(summaries, fileSummary{
			Path:       f.FilePath,
			Symbols:    len(f.Symbols),
			Relevance:  f.Relevance,
			TokenCount: f.TokenCount,
		})
	}

	if summaries == nil {
		summaries = []fileSummary{}
	}

	data, _ := json.Marshal(map[string]interface{}{
		"files":       summaries,
		"total_files":  stats.TotalFiles,
		"total_symbols": stats.TotalSymbols,
		"cached_files": stats.CachedFiles,
	})
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	return result, nil
}
