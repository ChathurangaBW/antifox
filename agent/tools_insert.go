package agent

import (
	"context"
	"encoding/json"
	"strings"

	"antifox/db"
	actool "antifox/engine/tool"
	"antifox/knowledge"
)

// =====================================================================
// Unified asset insertion tools
// =====================================================================

// SetAssetStore wires the asset store and company store onto this ToolSet
// so the insert_assets, add_company_scope, and list_assets tools are active.
func (t *ToolSet) SetAssetStore(as *db.AssetStore, cs *db.CompanyStore) {
	t.as = as
	t.cs = cs
	if t.kb == nil && as != nil { // server-level ToolSet (nil ExplorationStore): wire KB off the asset store's DB
		t.kb = knowledge.NewService(as.DB())
	}
}

// assetInputItem is one element of the insert_assets "assets" array.
type assetInputItem struct {
	Type string `json:"type"` // root_domain|ip|subdomain|app|service|endpoint

	// ---- root_domain / subdomain ----
	Domain      string   `json:"domain"`
	ICP         string   `json:"icp"`
	RecordType  string   `json:"record_type"`
	RecordValue []string `json:"record_value"`

	// ---- ip ----
	IP           string           `json:"ip"`
	BoundDomains []string         `json:"bound_domains"`
	OpenPorts    []db.PortService `json:"open_ports"`

	// ---- app ----
	AppName     string `json:"app_name"`
	BundleID    string `json:"bundle_id"`
	Category    string `json:"category"`
	Description string `json:"description"`
	AppICP      string `json:"app_icp"`
	CompanyID   *int64 `json:"company_id"` // explicit company link (app only; others auto-attribute via scope)

	// ---- service (http) ----
	URL           string           `json:"url"`
	Technologies  []string         `json:"technologies"`
	StatusCode    *int             `json:"status_code"`
	ContentLength *int64           `json:"content_length"`
	PageTitle     string           `json:"page_title"`
	FaviconMMH3   string           `json:"favicon_mmh3"`
	Auth          []map[string]any `json:"auth"`
	ServiceName   string           `json:"service_name"`
	ServiceIP     string           `json:"service_ip"` // optional enrichment IP

	// ---- service (other) ----
	Port  int    `json:"port"`
	Proto string `json:"proto"`

	// ---- endpoint ----
	Method string           `json:"method"`
	Params []map[string]any `json:"params"`
}

// insertAssets is the unified insert_assets agent tool.
func (t *ToolSet) insertAssets() actool.CoreTool {
	return writeTool(
		"insert_assets",
		"When new assets are discovered, insert them into the asset library in bulk immediately. A single submission may contain multiple asset types: root_domain/ip/subdomain/app/service/endpoint.\n"+
			"Fields for each asset type:\n"+
			"• root_domain: domain (root domain name, required), icp (registration, optional)\n"+
			"• ip: ip (IP address, required), bound_domains ([domain name]), open_ports ([{port,service}])\n"+
			"• subdomain: domain (subdomain name, required), record_type (A/AAAA/CNAME and so on), record_value (record value), icp (optional)\n"+
			"• app: app_name (application name, required), bundle_id (bundle id, optional), category, description, app_icp, company_id (owning company id, optional)\n"+
			"• service (http): url (required), technologies ([fingerprint]), status_code, content_length, page_title, favicon_mmh3, auth ([{type,username,password,...}]), service_ip\n"+
			"• service (other): service_name (required), ip or domain (at least one), port (required), auth ([...])\n"+
			"• endpoint: url (required), method (required), params ([{location,name,value,type}]), service_ip\n"+
			"auth/technologies/params are all merged by appending; they never overwrite the original values.\n"+
			"Returns: {results:[{index,id,type}], errors:[{index,error}]}",
		obj(map[string]any{
			"task_id": intp("Tasks id(Optional, for association task_ids)"),
			"assets": map[string]any{
				"type":        "array",
				"description": "Array of assets; each element is one asset record",
				"items": obj(map[string]any{
					"type": map[string]any{
						"type":        "string",
						"enum":        []string{"root_domain", "ip", "subdomain", "app", "service", "endpoint"},
						"description": "Asset type",
					},
					// root_domain / subdomain
					"domain":      str("Root domain or subdomain name (required for the root_domain/subdomain types)"),
					"icp":         str("ICP registration number (optional)"),
					"record_type": str("DNS record type: A/AAAA/CNAME/MX and so on (optional for subdomain)"),
					"record_value": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "List of DNS resolution values (optional for subdomain), e.g. [\"1.2.3.4\",\"2.3.4.5\"]",
					},
					// ip
					"ip": str("IP address (required for the ip type; also fill it in for service/endpoint types to associate an IP)"),
					"bound_domains": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "List of domain names bound to this IP (optional for the ip type)",
					},
					"open_ports": map[string]any{
						"type":        "array",
						"description": "List of open ports (optional for the ip type)",
						"items": obj(map[string]any{
							"port":    intp("Port Number"),
							"service": str("Service name, e.g. http/ssh/mysql and so on (optional)"),
						}, "port"),
					},
					// app
					"app_name":    str("Application name (required for the app type)"),
					"bundle_id":   str("Bundle ID (optional for the app type)"),
					"category":    str("Application category (optional)"),
					"description": str("Application description (optional)"),
					"app_icp":     str("Application ICP registration number (optional)"),
					"company_id":  intp("Membership id(app Type options;app I can't. scope Automatic attribution requires explicit designation.id by add_company_scope Back)"),
					// service (http)
					"url":         str("Full URL including protocol and port (required for HTTP services; service_type is set to http automatically)"),
					"status_code": intp("HTTP Response status code, for example 200/301/403/404(Optional)"),
					"content_length": map[string]any{
						"type":        "integer",
						"description": "HTTP response body size in bytes (optional)",
					},
					"page_title":   str("Page <title> content (optional)"),
					"favicon_mmh3": str("favicon MMH3 hash (optional)"),
					"technologies": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Fingerprint / technology stack list, e.g. [\"Nginx\",\"Vue\",\"Bootstrap\"] (optional)",
					},
					"auth": map[string]any{
						"type":        "array",
						"description": "List of discovered credentials, each entry containing type/username/password and similar fields (optional; appended, does not overwrite)",
						"items":       map[string]any{"type": "object"},
					},
					// service (other,Not HTTP)
					"service_name": str("Service name, e.g. ssh/mysql/redis (required when the service type is not HTTP)"),
					"port":         intp("Port number (required when the service type is not HTTP)"),
					// endpoint
					"method": str("HTTP method: GET/POST/PUT/PATCH/DELETE and so on (required for endpoint)"),
					"params": map[string]any{
						"type":        "array",
						"description": "List of request parameters, each entry containing location(query/body/header/path)/name/value/type (optional; appended, does not overwrite)",
						"items":       map[string]any{"type": "object"},
					},
				}, "type"),
			},
		}, "assets"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			if t.as == nil {
				return actool.Errorf("insert_assets Not enabled: AssetStore Not initialized"), nil
			}
			var a struct {
				Assets []assetInputItem `json:"assets"`
				TaskID int64            `json:"task_id"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return actool.Errorf("invalid input: " + err.Error()), nil
			}
			// use ToolSet.taskID as fallback
			taskID := a.TaskID
			if taskID == 0 {
				taskID = t.taskID
			}

			type result struct {
				Index int    `json:"index"`
				ID    int64  `json:"id"`
				Type  string `json:"type"`
			}
			type errEntry struct {
				Index int    `json:"index"`
				Error string `json:"error"`
			}

			var results []result
			var errs []errEntry

			for i, item := range a.Assets {
				typ := strings.TrimSpace(item.Type)
				var id int64
				var err error

				switch typ {
				case "root_domain":
					id, err = t.as.UpsertRootDomain(db.UpsertRootDomainReq{
						Domain: item.Domain,
						ICP:    item.ICP,
						TaskID: taskID,
					})

				case "ip":
					id, err = t.as.UpsertIP(db.UpsertIPReq{
						IP:           item.IP,
						BoundDomains: item.BoundDomains,
						OpenPorts:    item.OpenPorts,
						TaskID:       taskID,
					})

				case "subdomain":
					id, err = t.as.UpsertSubdomain(db.UpsertSubdomainReq{
						Domain:      item.Domain,
						RecordType:  item.RecordType,
						RecordValue: item.RecordValue,
						ICP:         item.ICP,
						TaskID:      taskID,
					})

				case "app":
					id, err = t.as.UpsertApp(db.UpsertAppReq{
						Name:        item.AppName,
						BundleID:    item.BundleID,
						Category:    item.Category,
						Description: item.Description,
						ICP:         item.AppICP,
						CompanyID:   item.CompanyID,
						TaskID:      taskID,
					})

				case "service":
					// distinguish HTTP vs other by presence of url
					if item.URL != "" {
						// agent may send "ip" or "service_ip" for the enrichment IP; accept both
						svcIP := item.ServiceIP
						if svcIP == "" {
							svcIP = item.IP
						}
						id, err = t.as.UpsertHTTPService(db.UpsertHTTPServiceReq{
							URL:           item.URL,
							Technologies:  item.Technologies,
							StatusCode:    item.StatusCode,
							ContentLength: item.ContentLength,
							PageTitle:     item.PageTitle,
							FaviconMMH3:   item.FaviconMMH3,
							Auth:          item.Auth,
							IP:            svcIP,
							TaskID:        taskID,
						})
					} else {
						id, err = t.as.UpsertOtherService(db.UpsertOtherServiceReq{
							Domain:      item.Domain,
							IP:          item.IP,
							Port:        item.Port,
							ServiceName: item.ServiceName,
							Auth:        item.Auth,
							TaskID:      taskID,
						})
					}

				case "endpoint":
					id, err = t.as.UpsertEndpoint(db.UpsertEndpointReq{
						URL:    item.URL,
						Method: item.Method,
						Params: item.Params,
						IP:     item.ServiceIP,
						TaskID: taskID,
					})

				default:
					errs = append(errs, errEntry{Index: i, Error: "unknown type: " + typ})
					continue
				}

				if err != nil {
					errs = append(errs, errEntry{Index: i, Error: err.Error()})
					continue
				}
				results = append(results, result{Index: i, ID: id, Type: typ})
				t.writes.Assets++
				t.anchorOwner(id)
			}

			return jsonResult(map[string]any{
				"results": results,
				"errors":  errs,
			})
		},
	)
}

// addCompanyScope writes to company_scope table and triggers asset attribution.
func (t *ToolSet) addCompanyScope() actool.CoreTool {
	return writeTool(
		"add_company_scope",
		"Add a root domain, IP, or IP range (CIDR) to a company's asset scope — once added, the system automatically claims every matching asset (existing and future).\n"+
			"company: the company name is unique; a new company is created if it does not exist, otherwise it is reused (the scope is simply merged in).\n"+
			"scope: one entry per line; supports a root domain name, a single IP, or a CIDR range (e.g. 1.2.3.0/24).\n"+
			"reason is required: describe the basis for attribution (whois, certificate, ASN, and so on).\n"+
			"Guardrail: bare TLDs and overly broad ranges are rejected (IPv4<=/16, IPv6<=/32); invalid entries are skipped and returned in errors.",
		obj(map[string]any{
			"company": str("Company name (new, existing, or reused; name only)"),
			"scope":   str("Asset scope, one entry per line: root domain / IP / CIDR"),
			"reason":  str("Basis for attribution (evidence/source) — always required"),
			"logo":    str("Company logo URL (optional; only effective when a new company is created)"),
		}, "company", "scope"),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			if t.cs == nil {
				return actool.Errorf("add_company_scope Not enabled: CompanyStore Not initialized"), nil
			}
			var a struct {
				Company string `json:"company"`
				Scope   string `json:"scope"`
				Reason  string `json:"reason"`
				Logo    string `json:"logo"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return actool.Errorf(err.Error()), nil
			}
			if strings.TrimSpace(a.Company) == "" {
				return actool.Errorf("company Can't be empty."), nil
			}
			companyID, _, err := t.cs.UpsertCompany(a.Company, a.Logo)
			if err != nil {
				return actool.Errorf("Create/Getting the company failed: " + err.Error()), nil
			}
			lines := splitLines(a.Scope)
			added, skipped, invalid, errMsgs := t.cs.AddScope(companyID, lines, a.Reason)
			out := map[string]any{
				"company_id": companyID,
				"added":      added,
				"skipped":    skipped,
				"invalid":    invalid,
			}
			if len(errMsgs) > 0 {
				out["errors"] = errMsgs
			}
			return jsonResult(out)
		},
	)
}

// listAssets lets an agent query the asset table.
func (t *ToolSet) listAssets() actool.CoreTool {
	return readTool(
		"list_assets",
		"Search for asset bank. Support DSL Organisation id/ids Directly, page break.\n"+
			"DSL Syntax:field=value Blur(ILIKE) | field==value Exact | field!=value Exclude | port>80 Number Comparison | Naked words.=The whole text is vague.\n"+
			"Logical operation:AND / OR(Keywords,AND Priority above OR),parenthesis grouping.\n"+
			"Use separate asset types type Parameter filter,DSL Not included type field.\n"+
			"Constraints: Unheard id/ids Time dsl It has to be empty.——Unconditional access to all assets is not allowed and must be conditional.\n"+
			"field:\n"+
			"  domain      Domain name (root domain / subdomain / service domain name)\n"+
			"  root_domain Root domain name (subdomain-only / service assets available)\n"+
			"  ip          IPv4/IPv6 Address\n"+
			"  url         Full URL (service / endpoint)\n"+
			"  page_title  Page title (HTTP services)\n"+
			"  icp         ICP registration number (root domain)\n"+
			"  service_name Name of the service (non-HTTP services)\n"+
			"  app_name    Application name (app type)\n"+
			"  method      HTTP method (endpoint type, e.g. GET/POST)\n"+
			"  service_type HTTP service type: http|other\n"+
			"  record_type DNS record type (subdomain name, such as A/CNAME)\n"+
			"  technology  Fingerprint / technology stack (numeric fields: = fuzzy, == exact)\n"+
			"  port        Port number (integer; supports > >= < <=)\n"+
			"  status_code HTTP status code (integer; supports > >= < <=)\n"+
			"  company_id  Owning company id (integer)\n"+
			"  task_id     Source task id (integer)\n"+
			"Example:status_code>=400 AND technology=shiro\n"+
			"      method==POST AND url=/api/admin\n"+
			"      icp=Beijing OR icp=Shanghai\n"+
			"      (port==80 OR port==443) AND technology=nginx",
		obj(map[string]any{
			"dsl":    str(`DSL query expression; see the tool description. Required when neither id nor ids is supplied. Example: status_code>=400 technology=shiro / method==POST url=/api/admin`),
			"type":   str("Asset type filter: root_domain|ip|subdomain|app|service|endpoint (an independent field that can be combined with dsl; type on its own is not enough to run a query, so dsl is still required)"),
			"id":     intp("Fetch a single asset directly by ID (optional; mutually exclusive with dsl/type)"),
			"ids":    map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "Fetch multiple assets directly by ID (optional; mutually exclusive with dsl/type)"},
			"limit":  intp("Maximum number of rows to return, default 10 (optional)"),
			"offset": intp("Pagination offset, default 0 (optional)"),
		}),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			if t.as == nil {
				return actool.Errorf("list_assets Not enabled: AssetStore Not initialized"), nil
			}
			var a struct {
				DSL    string  `json:"dsl"`
				Type   string  `json:"type"`
				ID     int64   `json:"id"`
				IDs    []int64 `json:"ids"`
				Limit  int     `json:"limit"`
				Offset int     `json:"offset"`
			}
			_ = json.Unmarshal(in, &a)
			if a.Limit <= 0 {
				a.Limit = 10
			}

			var assets []*db.Asset
			var err error
			switch {
			case a.ID > 0:
				assets, err = t.as.GetByIDs([]int64{a.ID})
			case len(a.IDs) > 0:
				assets, err = t.as.GetByIDs(a.IDs)
			case a.DSL != "":
				assets, err = t.as.QueryDSL(a.DSL, a.Type, a.Limit, a.Offset)
			default:
				return actool.Errorf("Unsigned id/ids Time dsl Unable to be empty: Unconditional search of all assets is not allowed, please provide conditions for searching"), nil
			}
			if err != nil {
				return actool.Errorf("DSL Error: " + err.Error()), nil
			}
			return jsonResult(map[string]any{
				"count":  len(assets),
				"assets": assets,
			})
		},
	)
}

// listCompanies lets an agent enumerate companies (Enterprise) with their scope + asset count.
func (t *ToolSet) listCompanies() actool.CoreTool {
	return readTool(
		"list_companies",
		"List the companies in the asset library along with their asset scope and the number of attributed assets. Use it to see which companies exist and to obtain their company_id"+
			"(used when associating an app in insert_assets and when filtering by company_id in list_assets)."+
			"Optional search filters by company name (case-insensitive); leave it empty to return all.",
		obj(map[string]any{
			"search": str("Filter by company name (optional, case-insensitive); leave empty to return all"),
		}),
		func(_ context.Context, in json.RawMessage) (actool.Result, error) {
			if t.cs == nil {
				return actool.Errorf("list_companies Not enabled: CompanyStore Not initialized"), nil
			}
			var a struct {
				Search string `json:"search"`
			}
			_ = json.Unmarshal(in, &a)
			cos, err := t.cs.ListCompanies()
			if err != nil {
				return actool.Errorf("Failed to query company: " + err.Error()), nil
			}
			q := strings.ToLower(strings.TrimSpace(a.Search))
			type companyOut struct {
				ID         int64    `json:"id"`
				Name       string   `json:"name"`
				AssetCount int      `json:"asset_count"`
				Scope      []string `json:"scope"`
			}
			out := make([]companyOut, 0, len(cos))
			for _, c := range cos {
				if q != "" && !strings.Contains(strings.ToLower(c.Name), q) {
					continue
				}
				scope := make([]string, 0, len(c.Scope))
				for _, r := range c.Scope {
					scope = append(scope, r.Raw)
				}
				out = append(out, companyOut{ID: c.ID, Name: c.Name, AssetCount: c.AssetCount, Scope: scope})
			}
			return jsonResult(map[string]any{"count": len(out), "companies": out})
		},
	)
}

// splitLines splits a multi-line string into non-empty trimmed lines.
func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// WorkerTools returns the tool set for a work agent.
func (t *ToolSet) WorkerTools() []actool.CoreTool {
	return []actool.CoreTool{
		t.listFindings(), t.listFacts(), t.nodeDetail(),
		t.addFinding(), t.recordFact(),
		// cross-work retrieval
		t.searchAllWorkerTraces(), t.listWorkerTraces(), t.getWorkerTrace(),
		// asset management (handlers guard nil store internally)
		t.insertAssets(), t.addCompanyScope(), t.listAssets(), t.listCompanies(),
		// knowledge base (global + this task); nil store guarded internally
		t.searchKnowledge(), t.listKnowledgeCategories(),
	}
}

// MainAgentTools returns the human-interface tool set.
func (t *ToolSet) MainAgentTools() []actool.CoreTool {
	return []actool.CoreTool{
		t.graphOverview(), t.listFindings(), t.listFacts(), t.nodeDetail(),
		t.getWorkerOutput(), t.getWorkerTrace(), t.searchAllWorkerTraces(), t.addHint(), t.addIntent(),
		// asset management (handlers guard nil store internally)
		t.insertAssets(), t.addCompanyScope(), t.listAssets(),
		// knowledge base (global + this task)
		t.searchKnowledge(), t.listKnowledgeCategories(),
	}
}

// AllDomainTools returns the union of all domain tools across all agent types,
// deduped by name (mainagent order wins). Used by the server to build a registry
// for injecting domain tools into agents (Auto, custom) that don't own a per-task
// ToolSet. The caller provides real stores; tools are callable at taskID=0 scope.
func (t *ToolSet) AllDomainTools() []actool.CoreTool {
	seen := map[string]bool{}
	var out []actool.CoreTool
	all := append(append(t.MainAgentTools(), t.PlannerTools()...), t.WorkerTools()...)
	for _, tool := range all {
		if !seen[tool.Name()] {
			seen[tool.Name()] = true
			out = append(out, tool)
		}
	}
	return out
}
