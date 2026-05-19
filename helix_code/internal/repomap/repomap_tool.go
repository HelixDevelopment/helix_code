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

func (t *RepoMapTool) Name() string { return "repomap" }

// Description resolves the user-facing tool description through the
// CONST-046 Translator seam (round-198 §11.4 anti-bluff sweep,
// 2026-05-19). The default NoopTranslator returns the raw message ID
// "internal_repomap_tool_description" — loud-echo failure mode by
// design so silent translation loss surfaces immediately. helix_code
// wires a real *i18nadapter.Translator at boot via SetTranslator.
func (t *RepoMapTool) Description() string {
	return tr(context.Background(), "internal_repomap_tool_description", nil)
}
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
