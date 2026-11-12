package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	actool "antifox/engine/tool"

	"antifox/knowledge"
)

// SetKnowledgeStore wires the knowledge-base service so search_knowledge /
// list_knowledge_categories are active. nil = the tools report "not enabled".
func (t *ToolSet) SetKnowledgeStore(kb *knowledge.Service) { t.kb = kb }

// search_knowledge — semantic (or keyword-fallback) retrieval over the two-layer
// knowledge base: global common knowledge + this task's uploaded background.
func (t *ToolSet) searchKnowledge() actool.CoreTool {
	return readTool("search_knowledge",
		"Retrieving the knowledge base: global knowledge(Hole/0day/Skills/Methodology) + Current Task Exclusive Information(Rating rules/Scope of authorization/Objective/Checkpoint). Describe what you're looking for in natural languages and return the most relevant knowledge clips. Mandates should proactively retrieve knowledge to guide methodological selection and determination criteria.",
		obj(map[string]any{
			"query": str("Questions or themes to search, natural language description"),
			"top_k": intp("Returns the number of bars, default 5"),
		}, "query"),
		func(ctx context.Context, in json.RawMessage) (actool.Result, error) {
			var a struct {
				Query string `json:"query"`
				TopK  int    `json:"top_k"`
			}
			_ = json.Unmarshal(in, &a)
			if strings.TrimSpace(a.Query) == "" {
				return actool.Errorf("query Can't be empty."), nil
			}
			if t.kb == nil {
				return actool.Text("The knowledge base is not enabled."), nil
			}
			if a.TopK <= 0 {
				a.TopK = 5
			}
			hits, mode, err := t.kb.SearchAndLog(ctx, a.Query, t.taskID, a.TopK, t.worker)
			if err != nil {
				return actool.Errorf(fmt.Sprintf("Search Failed: %v", err)), nil
			}
			if len(hits) == 0 {
				return actool.Text("No relevant information was found in the knowledge base."), nil
			}
			var b strings.Builder
			fmt.Fprintf(&b, "Search Mode: %s,Hit. %d Article:\n", mode, len(hits))
			for i, h := range hits {
				scope := "Global"
				if h.Scope == "task" {
					scope = "Task exclusive"
				}
				fmt.Fprintf(&b, "\n--- Result %d (Similarity %.0f%%, %s) ---\n[%s] %s\n%s\n",
					i+1, h.Score*100, scope, h.Category, h.Title, strings.TrimSpace(h.Content))
			}
			return actool.Text(b.String()), nil
		})
}

// list_knowledge_categories — enumerate categories so the agent can browse before
// a targeted search_knowledge call.
func (t *ToolSet) listKnowledgeCategories() actool.CoreTool {
	return readTool("list_knowledge_categories",
		"List existing classifications of the knowledge base (global) + Current mission. Let's see what class we'll use. search_knowledge Targeted search.",
		obj(map[string]any{}),
		func(ctx context.Context, _ json.RawMessage) (actool.Result, error) {
			if t.kb == nil {
				return actool.Text("The knowledge base is not enabled."), nil
			}
			cats, err := t.kb.Categories(t.taskID)
			if err != nil {
				return actool.Errorf(fmt.Sprintf("Reading Failed: %v", err)), nil
			}
			if len(cats) == 0 {
				return actool.Text("The knowledge base is not classified."), nil
			}
			return actool.Text("Knowledge base classification:\n- " + strings.Join(cats, "\n- ")), nil
		})
}
