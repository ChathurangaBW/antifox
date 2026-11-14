package server

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"sort"

	"antifox/agent"
)

// Tool definition upgrades that preserve operator edits.
//
// A built-in tool's description and schema are seeded into the tools table
// once, and the stored row then overrides the code: operators edit both in the
// UI. Shipping a new parameter therefore needs a migration, and that migration
// must tell the shipped text apart from an operator's.
//
// The previous version decided by description prefix and replaced the whole
// definition: a customized description that kept the stock opening words was
// overwritten, schema-only edits were lost, and a current description with a
// stale schema was marked done without any upgrade. This version compares
// against every definition the tool has actually shipped with
// (stockdefs/<key>.json, generated from each release tag and upstream) and
// decides per part:
//
//   - description: replaced only if it exactly equals a shipped description
//     (or is empty); otherwise kept.
//   - each schema property: replaced only if it exactly equals that property in
//     some shipped version; otherwise kept. Properties the current code adds
//     are added; properties an operator added are kept.
//   - other schema keywords (required, type, ...): same rule as properties.
//
// The write is conditional on the row being unchanged since it was read, and
// completion is recorded only after re-reading the row and confirming every
// current property is present.

//go:embed stockdefs/*.json
var stockDefsFS embed.FS

// stockToolDef is one shipped definition of a built-in tool.
type stockToolDef struct {
	Version     string         `json:"version"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"schema"`
}

// loadStockToolDefs reads the shipped definitions recorded for key.
func loadStockToolDefs(key string) ([]stockToolDef, error) {
	b, err := stockDefsFS.ReadFile("stockdefs/" + key + ".json")
	if err != nil {
		return nil, err
	}
	var defs []stockToolDef
	if err := json.Unmarshal(b, &defs); err != nil {
		return nil, err
	}
	return defs, nil
}

// toolUpgrade is the computed result of merging a stored definition with the
// current code definition.
type toolUpgrade struct {
	Description string
	Schema      map[string]any
	Changed     bool
	Kept        []string // operator-owned parts left as they were, for the log
}

// jsonNormalize round-trips v through JSON so values read from Go literals and
// from the database compare equal (numbers become float64, slices []any).
func jsonNormalize(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if json.Unmarshal(b, &out) != nil {
		return v
	}
	return out
}

// mergeToolDefinition computes the upgraded definition. It is pure, so every
// case can be tested without a database.
func mergeToolDefinition(curDesc string, curSchema map[string]any, seedDesc string, seedSchema map[string]any, stock []stockToolDef) toolUpgrade {
	seed, _ := jsonNormalize(seedSchema).(map[string]any)
	cur, _ := jsonNormalize(curSchema).(map[string]any)
	if cur == nil {
		cur = map[string]any{}
	}
	var stockSchemas []map[string]any
	stockDescs := map[string]bool{"": true}
	for _, s := range stock {
		stockDescs[s.Description] = true
		if m, ok := jsonNormalize(s.Schema).(map[string]any); ok {
			stockSchemas = append(stockSchemas, m)
		}
	}
	isStockValue := func(path func(map[string]any) (any, bool), v any) bool {
		for _, s := range stockSchemas {
			if sv, ok := path(s); ok && reflect.DeepEqual(sv, v) {
				return true
			}
		}
		return false
	}

	up := toolUpgrade{Description: curDesc}
	if stockDescs[curDesc] {
		up.Description = seedDesc
	} else if curDesc != seedDesc {
		up.Kept = append(up.Kept, "description")
	}

	out := map[string]any{}
	// Top-level keywords other than properties.
	for k, v := range cur {
		if k == "properties" {
			continue
		}
		key := k
		if isStockValue(func(m map[string]any) (any, bool) { v, ok := m[key]; return v, ok }, v) {
			if sv, ok := seed[k]; ok {
				out[k] = sv
			} // a stock keyword the current code dropped is dropped too
		} else {
			out[k] = v
			if sv, ok := seed[k]; !ok || !reflect.DeepEqual(sv, v) {
				up.Kept = append(up.Kept, k)
			}
		}
	}
	for k, sv := range seed {
		if k == "properties" {
			continue
		}
		if _, ok := cur[k]; !ok {
			out[k] = sv
		}
	}

	// Properties.
	curProps, _ := cur["properties"].(map[string]any)
	seedProps, _ := seed["properties"].(map[string]any)
	props := map[string]any{}
	for name, v := range curProps {
		pname := name
		stockProp := isStockValue(func(m map[string]any) (any, bool) {
			ps, _ := m["properties"].(map[string]any)
			pv, ok := ps[pname]
			return pv, ok
		}, v)
		switch sv, inSeed := seedProps[name]; {
		case stockProp && inSeed:
			props[name] = sv
		case stockProp && !inSeed:
			// shipped earlier, removed by the current code: drop it.
		default:
			props[name] = v
			if !inSeed || !reflect.DeepEqual(sv, v) {
				up.Kept = append(up.Kept, "properties."+name)
			}
		}
	}
	for name, sv := range seedProps {
		if _, ok := curProps[name]; !ok {
			props[name] = sv
		}
	}
	out["properties"] = props
	up.Schema = out
	up.Changed = up.Description != curDesc || !reflect.DeepEqual(out, cur)
	sort.Strings(up.Kept)
	return up
}

// missingSeedProperties lists current-code properties absent from schema.
func missingSeedProperties(schema map[string]any, seedSchema map[string]any) []string {
	have, _ := schema["properties"].(map[string]any)
	want, _ := jsonNormalize(seedSchema).(map[string]any)
	wantProps, _ := want["properties"].(map[string]any)
	var missing []string
	for name := range wantProps {
		if _, ok := have[name]; !ok {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing
}

// upgradeBuiltinToolDefinition applies mergeToolDefinition to the stored row of
// key and verifies the result. It returns nil only when the stored row now
// carries every current property, so the caller may record completion.
func (s *Server) upgradeBuiltinToolDefinition(key string) error {
	var seed *agent.ToolSeed
	for _, sd := range agent.BuiltinToolSeeds() {
		if sd.Key == key {
			cp := sd
			seed = &cp
			break
		}
	}
	if seed == nil {
		return fmt.Errorf("no built-in tool %q", key)
	}
	stock, err := loadStockToolDefs(key)
	if err != nil {
		return fmt.Errorf("load shipped definitions: %w", err)
	}
	cur, err := s.m.pg.GetTool(key)
	if err != nil {
		return fmt.Errorf("read tool: %w", err)
	}
	if cur == nil {
		return fmt.Errorf("tool %q is not seeded yet", key) // retried on the next start
	}
	var curSchema map[string]any
	if len(cur.Schema) > 0 {
		if err := json.Unmarshal(cur.Schema, &curSchema); err != nil {
			return fmt.Errorf("stored schema is not a JSON object: %w", err)
		}
	}
	up := mergeToolDefinition(cur.Description, curSchema, seed.Desc, seed.Schema, stock)
	if up.Changed {
		newSchema, err := json.Marshal(up.Schema)
		if err != nil {
			return err
		}
		ok, err := s.m.pg.UpdateToolDefinitionIfUnchanged(key, cur.Description, cur.Schema, up.Description, newSchema)
		if err != nil {
			return fmt.Errorf("write tool: %w", err)
		}
		if !ok {
			return fmt.Errorf("tool %q changed while upgrading; will retry on the next start", key)
		}
		if len(up.Kept) > 0 {
			log.Printf("[tools] upgraded %s, kept operator-edited parts: %v", key, up.Kept)
		} else {
			log.Printf("[tools] upgraded %s to the current definition", key)
		}
	}
	// Postcondition, independent of the description: the stored schema must
	// carry every property the current code defines.
	after, err := s.m.pg.GetTool(key)
	if err != nil || after == nil {
		return fmt.Errorf("re-read tool after upgrade: %v", err)
	}
	var afterSchema map[string]any
	_ = json.Unmarshal(after.Schema, &afterSchema)
	if missing := missingSeedProperties(afterSchema, seed.Schema); len(missing) > 0 {
		return fmt.Errorf("tool %q still lacks properties %v after upgrade", key, missing)
	}
	return nil
}

// markMigrationDone records a migration flag and reports a failed write instead
// of discarding it. An unrecorded flag only means the idempotent migration runs
// again on the next start.
func (s *Server) markMigrationDone(flag string) {
	if err := s.m.pg.SetSetting(flag, "true"); err != nil {
		log.Printf("[migration] could not record %s, it will run again on the next start: %v", flag, err)
	}
}

// migrationDone reports whether flag is recorded.
func (s *Server) migrationDone(flag string) bool {
	v, _, _ := s.m.pg.GetSetting(flag)
	return v == "true"
}
