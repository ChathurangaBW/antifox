---
name: api-recon
Decryption: Call this skill when collecting the API interface.
---

# API Recon (front interface reconnaissance)

In the context of **already authorized**, as complete as possible:**backend API**(paths, methods, parameters, responses),**front end route route**,**UI functional trigger**(Tab, window, table operation, etc.).

---

## Border and Prohibition (Agent must read and violate and cross the border)

This skill **does only API/ parameter surface reconnaissance**and is not a leak excavation or infiltration phase.

### Task Boundaries

| Scope | Allowed | Prohibited |
|---|---|---|
| **Objective** | Embrace path, method, parameter, route, UI trigger point | SQLi/XSS/ ultra vires/explosive/fuzz leaks, repackaged attacks, destructive operations |
| **Authentication** | Use hooks plus stubs/mocks to bypass the **client-side** login gate | Request or guess user credentials, or submit a real login form |
| **When running** | Unsupported Hook interface to get SPA into the backshell | Process to continue by relying on a real back-end session |

### No supporting dynamic analysis (Phase 3 default)

1. By intercepting and stubbing **login, permissions, menus, etc.** with `preload.js` / `runtime_harvest.js`;
2. **mock body returned to the business query interface**properly structured, service code successful, data can be empty **;
3. Enable the front end to still render a post-entry page in a no backend or 401 environment, triggering more XHR/fetch/WebSocket;
4. **Empty data, blank tables, place of UI are expected**- Do not turn to real login or bug testing for this purpose.

**One sentence**: Mount the front road route with a mock,**Only outbound request**; backend returns are not important and what interfaces will be sent **is important.

### Hard Prohibitions in the Workflow

| Prohibited | Alternative approaches |
|---|---|
| Phase 1 before completion of grep/curl/Read master enterry `index-*.js` extraction API path | Run, `OUTDIR/harvest_static.py`. |
| Handwritten `extract_apis.py` and a script to replace harvest | Change `OUTDIR/harvest_static.py` and run back. |
| The same grep/ command failed. | Change of strategy: read tool_logs, read changes, see reference |
| Skip the door lock A/B, run straight `scripts/` original | Copy to OUTDIR and change to target |
| Real username/ password, OTP, OAuth, etc. | stub/mock (see above) |
| Skip stub for "get real data" for overpowering/injecting tests | Record only outbound, recon border |
| Remove, export sensitive data, batch writing, etc. irreversible operations | Coverage clicks the same |
| Runtime+ Dynamic Enumeration not completed, claiming to have all pages and interfaces | See "finish definition" or label limits |
| Unfinished parameter trigger matrix + diff, claiming all parameters are captured | Phase 3b matrix + Phase 5 diff |
| Inference to be filled/ optional using a single rantime sample | Multiple sample diff or check rule/ error invert |

---

## Two-layer model + running mode

| Layer | Output | Upper limit |
|---|---|---|
| **Static** (JS bundle) | Complete endpoint paths, route drafts, candidate request-builder fields | No HTTP method; parameters require Phase 1b; runtime-assembled URLs are missed |
| **Run-time**(live session) | Method + body + Response + Dynamic URL + WS/SSE; Multiple Sample diff Completion Parameters | Pages need to be actually rendered before being requested; a single sample is not sufficient to be filled out/optional |

| Run Mode | Engine | Application |
|---|---|---|
|  **depth**  | `runtime_harvest.js` (Puppeteer) | API List, METHOD/params/ Response, WS/SSE, Recoverable Bulk Run |
|  **coverage**  |  browser + `preload.js`  | Point Tab/ window/table, function point cover deeper |
| **both** | Run depth first, then coverage | Most complete; takes the longest |

**Parameter methodology** (no universal script): use harvest/regex for paths; use **anchor-window expansion + UI binding chains + multi-sample diffs + error-driven inference** for parameters (see section J of [reference.md](reference.md) for grep recipes).

---

## Definition of Done

All meets to claim recon complete:

- [ ] **Static**: Phase 1 Harvey output `api_static.txt`, `routes.txt`, `js/`
- [ ] **Run-time**: at least one depth or coverage; coverage/both must **Hook Entry + Dynamic Element Ring**
- [ ] **Enter the shell**: accessing a business path does not redirect to `/login` (watch hash routes)
- [ ] **Arguments**: coverage/both finish parameters trigger matrix + `param_samples.json`; Phase 5 merge `params_merged.json`
- [ ] **Depth**(if the module page is blank): Phase 4 permission tree is restored and rerun until it appears **module level API**(not only locale/bootstrap)
- [ ] **Delivery**: Phase 5 full output (see Phase 5 output table); `insert_assets` written into service and end-point assets

---

## Scripts and Gates

`scripts/` is a reference template only; **do not** run the original unchanged as the final result.

**Rule**: read it first â†’ adapt it to the target â†’ write it to `OUTDIR` (for example `recon/`) â†’ record changes in `CHANGES.md`; if it does not fit, rewrite it using the methodology and borrow only its structure.

| Restricted. | When? | Reference Script OUTDIR copy | Frequent Changes |
|---|---|---|---|
| **A (static)** | After Phase 0, before the **first** harvest/spider run | `harvest_static.py` / `spider_mpa.py` | **Most sites can use the default regex**; change endpoint regexes, webpack/Vite `publicPath`, or MPA exclude/cookie settings only when the manifest or dialect does not match |
| **B (runtime)** | After Phase 2, before depth/coverage | `runtime_harvest.js` / `preload.js` + `config.json` | Cookie/localStorage keys, neutralize-success values, stubs, login regex, API prefixes, and hash/history |

**SPA mandatory order** (not interchangeable; phase order takes precedence over â€œexplore before scriptsâ€):

| Steps | Must | Prohibited |
|---|---|---|
| Phase 0 after completion | Next Bash = `python3 OUTDIR/harvest_static.py <URL> OUTDIR` | curl/grep/Read Main entry `index-*.js` (usually > 500KB); runtime setup uses `runtime_harvest.js` / `preload.js` + `config.json` |
| Restricted A | Copy Script Minor changes required Run immediately ** | Hand-picked API, then decide whether or not you're going to get it. |
| Phase 1 before completion | `wc -l` check output; 404 retry | Handwritten extract script; for undownloaded URL repeatedly grep |
| From Phase 1b onwards | grep only `OUTDIR/js/*.js` | Use the main bundle instead of harvest |

- Copy `harvest_static.py` (optional) regex **Run immediately**
- The curl master * grep many times * wrote provisional * last but not least
- **MPA**: Phase 0 Next Bash = `python3 OUTDIR/spider_mpa.py ...`

---

## Tool and Output Constraints

| Constraint | Description |
|---|---|
| Large Documents | `index-*.js` of 100KB **proscribes**Read/grep context; treated with OUTDIR script |
| grep output | Must use `\| head -20` or `-m 5`; keep only path summaries in the conversation, never paste bundle excerpts |
| Verify | With `wc -l`, `ls \| wc -l`; do not read whole directory |
| Regex first scout | Optional, 1, KB small chunk or HTML; formal static is based on harvest |
|  reference  | Formula/template/defective vision [reference.md] (reference. md), do not repeat the full text of the inline |

---

## Execution Roadmap

```
Phase 0 Classification + OUTDIR
== sync, corrected by elderman == @elder_man
  â†’ Phase 1b Parameter Reverse
  â†’ Phase 2 Three doors of authority. â†’ config.json
  â†’ Restricted. B â†’ Phase 3 When running + Array of parameters
Please 4 Mission Tree (If necessary.)
Phase 5: merge the results + insert_assets: batch-insert every discovered service and endpoint asset; never leave out an asset that was already found.
```

**The previous item is not complete and cannot enter the next Phase**.

1. [ ]**Phase 0**: primary probe SPA/MPA; create `OUTDIR` [Phase 0] (#phase-0 -- classification)
2. [ ]**Restricted A + Phase 1**: Copy Script immediate **harvest `wc -l` check [Phase 1] (#phase-1 -- static)
3. [ ]**Phase 1b**: anchor extension window + binding layer `param_candidates.json` [Phase 1b] (#phase-1b -- parameter reverse)
4. [ ]**Phase 2**: Three doors of authority `config.json` [Phase 2] (#phase-2 - Three doors of authority)
5. [ ]**Restricted B**: Adjusted Runtime Script [Phase 3] (#phase-3-Runtime)
6. [ ]**Phase 3**: depth / coverage / both; confirmation of shell; parameter trigger matrix `param_samples.json`
7. [ ]**Phase 4**(if necessary): permission tree catch stubs Phase 3 [Phase 4] (#page 4 -- permission tree restoration)
8. [ ]**Phase 5**: Consolidated output + report + `insert_assets` [Phase 5] (#phase-5 - merged and reported)

---

## Phase 0 â€” Classification

HTML,**Create `OUTDIR`**(do not change `scripts/` in skill):

- **SPA**: Empty shell + `<div id=app>` + chank Phase 1â€“ 5
- **MPA**: SSR + `<form>`, no endpoint bundle â€” **Gate A:**

```bash
python3 recon/spider_mpa.py <BASE_URL> <OUTDIR> [--cookie "session=..."] [--max 300] [--depth 5] [--exclude "logout|delete|destroy"]
```

Outputs `forms.txt`, `links.txt`, `api_inline.txt`. SPA if forms 0 Phase 1.

---

## Phase 1 â€” Static

Compliance with [scripts and door locks] (# scripts and door locks) Â· [tools and output constraints] (# tools and output constraints).

```bash
python3 recon/harvest_static.py <BASE_URL> <OUTDIR>
```

Harvest: parse the HTML scripts and the webpack/Vite manifest -> download all the lazy chunks -> output `js/`, `api_static.txt`, `routes.txt`, `chunkmap.txt`.

```bash
wc -l OUTDIR/api_static.txt OUTDIR/routes.txt
ls OUTDIR/js | wc -l
```

- cunk count vs manifest: 404 must be changed for retry, not manually curl cunk-by-man
- `api_static.txt` too little relaxed in OUTDIR endpoint rerun (see reference)

### Phase 1b â€” Parameter Reverse Engineering

Path from Phase 1; cross-references must be recon separately. grep rules see [tools and output constraints] (# tools and output constraints).

**Completion criterion**: Important interfaces can answer - field name, transmission location, type extrapolation, mandatory filling, sample value, confidence.

#### 1b.0 â€” Transport Form

| Form | Where the Parameter Is | For Static Analysis, Look First At |
|---|---|---|
|  REST JSON  |  body + query  | path By the anchor. `(params\|data\|body)\s*:\s*\{`  |
|  GraphQL  |  `variables`  | gql Templateã€`$page: Int` |
| Tradition form |  urlencoded  |  `<form>`ã€`FormData`  |
| File Upload |  multipart  |  `FormData.append`  |
| Path Parameters |  `/user/:id`  | Route Table + `useParams` / `$route.params` |
| Encryption/signature | Packed in `sign`/`data` | Hook encryption function entry (reference D) |

Output: Each interface is marked at `transport: query|json|form|graphql|encrypted`; record wrapper fields such as `data`, `variables`, and `bizData`.

#### 1b.1 â€” Anchor-Based Window Expansion

Find a group object for the extended window, anchored in a known path:

```bash
grep -n '"/api/user/list"' OUTDIR/js/*.js | head -20
grep -rhoaE '.{0,120}("/api[^"]+").{0,200}' OUTDIR/js/*.js | head -20
grep -rhoaE '(params|data|body|payload)\s*:\s*\{' OUTDIR/js/*.js | head -20
```

| The Wrapper Layer | Parameter Clues |
|---|---|
| axios Example |  `data` / `params`  |
| Unified | Interceptor in global field |
| OpenAPI Client | Generate method signature |
|  React Query / SWR  | hook Second parameter |
|  Vue composable  | composable Participation |

Type residual: `yup`/`zod`/rules, `Form.Item name=`, embedded Swagger.

~ `param_candidates.json`:`{ path, fields[], source: "static-callsite", confidence }` ~

#### 1b.2 â€” Binding Layer

```
Form field → onFinish/handleSubmit → transform → API payload
```

| Bind Source | How? |
|---|---|
| Form submit | With submit and transform and API. |
| Table search |  `getFieldsValue()` â†’ `params`  |
| Routes |  `:id` / `?tab=`  |
| Interceptor. | Global `tenantId`, page break, sign |
| Enumerate select | `options` API equator |

DevTools calls stick from `fetch`/`XHR.send` to the group package function.

#### 1b.3 - Group 3 Questions (Phase 2 Questions)

| Question. | What Must Be Answered |
|---|---|
| **Reassembly** | Where's Payload? |
| **Verify** |  requiredã€patternã€enum  |
| **Transfer** | path / query / body / multipart / Head |

Interceptor door (Phase 2) reads global injection fields (Authorization, `X-Tenant-Id`, sign).

#### 1b.4 â€” Link to Phase 3

Candidate fields are from static/string layers;**Phase 3 argument matrix + diff + Phase 5 error reverses.

---

## Phase 2 â€” The Three Authentication Gates

In `OUTDIR/js/` grep (with `head`), write `config.json` (reference):

| The door. | Problem | Keywords |
|---|---|---|
| ****Rendering Gate** | How do you judge that login? |  `isLogin`ã€`getToken`ã€Cookie/localStorage  |
| **Interceptor Gate** | What trigger jump `/login`? |  `response_code`ã€`errno`ã€axios interceptor  |
| **Content Gate** | Where does the menu/access come from? |  `menu`ã€`permission`ã€`role`ã€`acl`ã€`routes`  |

The name of the localStorage key is not allowed to be used as evidence - it must be confirmed from the chunk/ request chain.

**Export = Restricted B**: Conclusion reached at `config.json` and changed at `OUTDIR/runtime_harvest.js`/`preload.js`.

### Phase 2b â€” API observation (optional)

Confirm session names with OUTDIR in `preload.js`, Authorization, nested API URL:

| Configuration | Output |
|---|---|
| `recordDetail: true` | `__API_RECON_DETAIL__` |
|  `observe.xhrHeaders: true`  | headers Observation |
| `extractUrlsFromResponse: true` | Responses of the target API |
|  `observe.storageReads/cookieReads: true`  | Reload config |
| `neutralizeVueRouter: true` | `__API_RECON_ROUTES__` |

Export once per round: `__API_RECON_LOG__`, `__API_RECON_DETAIL__`, `__API_RECON_ROUTES__`, `__API_RECON_OBSERVE__`.

---

## Phase 3 â€” When running

B; comply with [border and prohibition] (#border and prohibition of agent - read-and-read - breach-and-cross-border) - unsupported mock strategy.

`config.json` Set-up `"runtimeMode": "depth" | "coverage" | "both"` (see reference for template).

### Hook with stub (deepth + coverage)

| Layer | Scope | Purpose |
|---|---|---|
| L1 Precision | auth/ permissions/bootstrap stub | First screen access. |
| L2 Negative fixes | Response to all JSON | No login code Success |
| L3 | `/api` et al. missed L1 | UI! UI! UI! |

- **deepth**: fake auth + `forward` recode + `stubs`; past `routes` (hash/history); output `runtime_api.json`
- **coverage**:**document-start**Injection `preload.js` (CDP `addScriptToEvaluateOnNewDocument` or Userscript)

Authentication: `window.__API_RECON_PRELOAD__` exists; business path does not return `/login`.

```bash
cd recon && npm install
node runtime_harvest.js config.json
```

### 3b â€” coverage dynamic enumerator (compulsory)

1. Main Navigator/ Sidebar - Every click, etc. 1â€“3s
2. Tab â€” `role=tab`ã€`.ant-tabs-tab`
3. Table - First Line View/edit/Detail
4. Toolbars - Export, Filter, New (**Avoid irreversible deletion**)
5. Each module -- merge API/ route
6. SPA - Controlled `routes.txt` without `pushState` covering the path (MPA banned)

**Arguments trigger matrix**(compulsory): each module is recorded once by type of operation,**diff multiple samples**:

| Operation | Normal excess parameters |
|---|---|
| List First Screen | Page Break + Default Filter |
| Point Search |  keywordã€filter  |
| Advanced Filter | Moreoptional |
| New/edit | Fullity |
| Batch/Export/Sort |  `ids[]`ã€`exportType`ã€`sortField`  |

**Stub under outboud body/headers remains true**- subject to request. Recorded at `scan_raw.json`, `param_samples.json`, `api_detail.json`.

- **Vue**: `neutralizeVueRouter: true` + document-start preload
- **React**: `routes.txt` + sidebar click + `pushState`
- **both**: 3a depth, 3b coverage

---

## Phase 4 â€” Permission Tree Restoration

**Trigger**: Modular page blanks / only bootstrap (e.g. locale) content door not over.

| Symptom | Meaning |
|---|---|
| Shell Entry Succeeded | Render door + Interceptor door passed. |
| Sidebar Shortcut/Blank Click | stubshape or incomplete permissions |
| Every route, API. Same and very few. | `v-if permission` not passed |
| `routes.txt` is much smaller than the bundle | Must be supplemented from the auth module |

```bash
grep -rhoaE '"/api[^"]*(permission|perm|role|menu|acl)[^"]*"' OUTDIR/js/*.js | sort -u | head -30
grep -rhoaE 'userRouteAuth|getResultTree|routeMap|routeLink|menuList|authList' OUTDIR/js/*.js | head -20
```

Typical chains: `role_permissions`(flat codes) + `permissions/all`(tree) `getResultTree` `userRouteAuth[CODE].url`.

```bash
python3 recon/extract_route_map.py recon/js recon/
python3 recon/build_perm_tree.py recon/js recon/ --config recon/config.json
```

Intermediate outputs: `route_map.json`, `userRouteAuth.json`, `permissions_tree.json`, `*_stub.json`, `perm_codes_all.txt`.

Stub check: The outer layer of `response_code` corresponds to the interceptor door; flat codes aligns with the tree; `routes` overwrites all `route_map` link.

Update `config.json` and **Runner Phase 3**. Large SPA can `waitUntil`, `routeTimeout`, `perRouteMs` (see reference A3/I).

---

## Phase 5 â€” Consolidation and reporting

### Output Table

| Documentation | Phase | Content |
|---|---|---|
| `js/`, `api_static.txt`, `routes.txt`, `chunkmap.txt` | 1 | Static bundle, bundle paths, and paths |
|  `param_candidates.json`  |  1b  | Static Parameter Field Candidates |
|  `config.json`  |  2  | Three doors + runtime configuration |
|  `runtime_api.json`  |  3a  | depth detailed recording (including WS/SSE) |
|  `param_samples.json`ã€`scan_raw.json`ã€`api_detail.json`  |  3b  | Multiple samples, click logs, detail |
| `route_map.json` et al. |  4  | permission tree middle file (if executed) |
|  `params_merged.json`  |  5  | Merge Reference & + Confidence |
| `api_merged.txt` | 5 | `METHOD /path [params] [static\|runtime\|both]` |
|  `site_map.json`  |  5  | Routes, zzozz, paraams, function points, limitations |
|  **insert_assets**  |  5  | Write all services, end-point assets into the asset pool |

### 5b â€” Parameter Merging

From `param_samples.json` diff,**there is no generic consolidated script**. The confidence rule is found in reference J7 (high/medium/low/to trigger).

### 5c â€” Reverse Inference From Errors

Within the authorized scope, incomplete requests that return 400 responses (for example `field 'x' is required` or enumeration errors) are parameter reconnaissance, not vulnerability testing. Account for `data` wrappers, `variables`, and pre-encryption `bizData`.

The report should indicate: runtimeMode, API number at static/operational times, parametric confidence, unoverrided modules, summary of `CHANGES.md` relative reference scripts.

`site_map.json` Proposed structure:

```json
{
  "site": "https://example.com",
  "runtimeMode": "both",
  "appType": "vue-spa",
  "routeGuardStrategy": ["nav-neutralize", "L1-auth", "L2-patch", "forward"],
  "apisFromStatic": [],
  "apisFromRuntime": [],
  "apis": [],
  "params": [{ "method": "POST", "path": "/api/user/list", "transport": "json", "fields": [] }],
  "frontendRoutes": [],
  "routesVerifiedByClick": [],
  "featuresTriggered": [],
  "limitations": ""
}
```

See [reference.md] (reference. md) for more fields and grep formula.

---

## General description

- **Frame irrelevant**: webpack/Vite/Angularlazyload method is the same
- **Transmission**: REST/JSON, GraphQL, WebSocket, SSE; gRPC-web not in range
- **SSR**: Client feedable; RSC/Server Actions not fully listable
- **Blind area**: JSVMP, WASM, HMAC/mTLS
- **Parameter blind area**: Conditional connection, Hidden Params, WASM packs "to trigger"/ "unreachable"
- **Static is a safety net**: rantime can still enumerate endpoint when blocked

---

## Additional resources

- Grep formula, `config.json` template, drainage, Hook, Parameter J, site_map template:**[reference.md] (reference. md)**
- See [script and door lock] (# script and door lock) table for reference script path
