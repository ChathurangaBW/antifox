# api-recon — Reference manual

Grep formula, `config.json` template and decomposition. All greps are performed for `js/` directories. Bundle can start with `js-beautify` or `sed 's/}/}\n/g'`, usually with the following window.

## Note by the Chair

All documents in `scripts/` are **reference template**, which must be adjusted to the target site prior to implementation. Typical change point:

| Script | Frequent adjustments required |
|---|---|
| `harvest_static.py` | Endpoint parsing (webpack/Vite manifest logic, publicPath, micro-frontends) |
| `runtime_harvest.js` | Neutralize field name and success, stub matching rules with body structure, rootes source, WS recording, `waitUntil`/`routeTimeout`/ `proxy` |
| `preload.js` | `loginPathRe`、L1 stubs、`neutralize.fields`、`apiPattern`、Whether or not to enable L3、`recordDetail`、`observe.*`、`neutralizeVueRouter` |
| `spider_mpa.py` | `--exclude` Disruptive Links, cookies, depth/max, Sphere Filter |
| `extract_route_map.py` | `routeMap`/ `routeLink` Regulation, KEY Naming Mode |
| `build_perm_tree.py` | `userRouteAuth` parsing, the `ROOTS`/`PREFIX_PARENT` hierarchy, and the stub field names |
| `config.json` | Unified entrance to all above site exclusive parameters |

The adjusted document is proposed in the list of tasks (e.g. `recon/`), with specific changes to the relative reference script indicated in the report.

---

## A. Reverse three doors.

### A1. Rendering doors — "How do you judge that login? "

```bash
grep -rhoaE '.{0,40}(isLogin|isAuthenticated|loggedIn|hasLogin|requireAuth)\b.{0,80}' js | head
grep -rhoaE 'function (getUser|getToken|getAuth)[0-9]?\([^)]*\)\{.{0,200}' js | head
grep -rhoaE '(localStorage|sessionStorage)\.getItem\("[^"]+"\)' js | sort -u
grep -rhoaE '(Cookies?|cookie)\.(get|load)\("[^"]+"\)' js | sort -u
grep -rhoaE '\batob\(|JSON\.parse\(|jwt|decode' js | head
```

Find links `isLogin = f(getUser())` `getUser = decode(storage.read(KEY))`, determine **Storage keys**,**Containers**(Cookie vs localStorage),**Encoding**:

| Encoding | Config Counterfeiting |
|---|---|
| Explicit Strings / `"1"` / token | `"value": "anything-truthy"` |
| `JSON.parse(x)` | `"value": "json:{\"id\":1,\"username\":\"admin\"}"` |
| `JSON.parse(atob(x))` | `"value": "b64json:{\"id\":1,\"username\":\"admin\"}"` |
| JWT | An unsigned (`alg:none`) JWT, or a JWT signed with the bundle's internal key |
| Encryption (SM2/AES/RSA) | Find hard encoded keys; rendering doors only needs to decode blob when forge; otherwise, static header |

→ Write `cookies` / `localStorage`。

### A2. Interceptor door -- "What trigger jump/login?"

```bash
grep -rhoaE '.{0,60}(interceptors\.response|axios|request\.use).{0,120}' js | head
grep -rhoaE '.{0,40}(response_code|errcode|errno|\bcode\b|\bret\b|\bstatus\b)\s*[=!]==?\s*[\-0-9]{1,4}.{0,60}' js | head -20
grep -rhoaE '.{0,40}(Not Login|Please re-log in.|Login expired|unauthorized|Login invalid|Authorization|token.{0,10}invalid).{0,40}' js | head
grep -rhoaE '.{0,30}(location\.href|router\.(push|replace)|navigate)\([^)]*login[^)]*\)' js | head
```

Determines:**field names**,**success**(usually `0` or `200`),**successful value to trigger a jump**. Validation with junk session:

```bash
curl -sk -X POST -H 'Cookie: <fakekey>=junk' https://target/api/<protected> -d '{}' -H 'Content-Type: application/json'
```

→ Write `neutralize.fields` + `neutralize.success`。

### A3. Content door - "Where do menus/accesss come from?"

```bash
grep -rhoaE '"/api[^"]*(permission|perm|role|menu|acl|resource|nav)[^"]*"' js | sort -u
grep -rhoaE '.{0,30}(menus|permissions|menuList|routeList|authList|role_permissions)\b.{0,120}' js | head
grep -rhoaE 'userRouteAuth|getResultTree|routeMap|routeLink|hasPermission|checkAuth' js | head
grep -rhoaE '([A-Z_][A-Z0-9_]*):\{name:"[^"]*",link:"/[^"]+"\}' js | head
```

**Two-storey data**(common business backstage):

| API | Typical Payload | Consumer |
|---|---|---|
| `.../role_permissions` | `{ permissions: string[], role_type }` | Route guard, button level ACL |
| `.../permissions/all` | `tree[{ code, position, children }]` | Sidebar menu rendering |
| Bundle `userRouteAuth` | `{ CODE: { url, name? } }` | Code & frontend |
| Bundle `routeMap` | `{ KEY: { name, link } }` | Alias parser (webpack `o.DASHBOARD`) |

Read consumer code confirmation: `getResultTree(tree, permissions)` how to filter, `v-if`/ `hasAuth(code)` check which field.

**Manual forge**(small site): build permissive payload `stubs`.

**Full rights tree returned**(large site, sidebar/submodule still blank): see section **I**.

---

## B. config.json Template

```json
{
  "baseUrl": "https://target/",
  "runtimeMode": "both",
  "chromium": "/usr/bin/chromium",

  "cookies": [
    { "name": "auth", "value": "b64json:{\"id\":1,\"username\":\"admin\",\"role\":\"admin\",\"func\":{},\"permissions\":[\"*\"]}" }
  ],
  "localStorage": { "token": "faketoken", "isLogin": "1" },

  "neutralize": {
    "fields": ["response_code", "code", "errno", "ret", "status"],
    "success": 0,
    "flags": { "success": true, "message": "ok" }
  },
  "forward": true,
  "loginUrlPattern": "/login",
  "apiPattern": "/api/|/rest/|/graphql",

  "mockTier": "L1+L2",
  "recordDetail": true,
  "observe": {
    "storageReads": false,
    "cookieReads": false,
    "xhrHeaders": true
  },
  "neutralizeVueRouter": true,
  "stubs": [
    {
      "match": "permissions/all|/menu|role_permissions",
      "body": {
        "response_code": 0, "code": 0,
        "data": {
          "permissions": ["*"],
          "menus": [
            { "name": "dashboard", "path": "/dashboard", "show": true, "children": [] },
            { "name": "alert", "path": "/alert", "show": true, "children": [] }
          ]
        }
      }
    }
  ],

  "explore": {
    "clickTabs": true,
    "clickTables": true,
    "pushStateFallback": true,
    "maxMenuItems": 50
  },

  "routes": ["/dashboard", "/alert", "/asset", "/device", "/report", "/config", "/system"],
  "waitMs": 1500, "perRouteMs": 900, "headless": true,
  "waitUntil": "domcontentloaded",
  "routeTimeout": 12000,
  "proxy": "",

  "captureResponses": true, "recordWs": true, "respMax": 600
}
```

Field description:
- `runtimeMode`:`depth` (Puppeteer), `coverage` (browser MCP), `both`
- `cookies[].value` prefix: `b64json:` base64 (JSON); `json:` original JSON; no prefix literal
- `forward: true` forwards the real request and rewrites the code field;`false` completely offline stub
- `mockTier`: coverage mode preload enable layers such as `L1+L2`, `L1+L2+L3`
- `routes` from `routes.txt`; extra `<a href>` after the `routes.txt`; forge menu
- `captureResponses` / `recordWs` only depth mode is valid
- `waitUntil`: Large SPA with `domcontentloaded`, avoid `networkidle2` hanging
- `routeTimeout`: Single route `page.goto` timeout (ms)
- `proxy`: Puppeteer `--proxy-server`; can also set `HTTP_PROXY`/ `HTTPS_PROXY`

### B1. Double stub template (role_permissions + permissions/all)

```json
"stubs": [
  {
    "match": "role_permissions",
    "body": {
      "response_code": 0,
      "data": {
        "permissions": ["MONITOR", "MONITOR_ALERT", "THREAT", "ASSETS_RISK"],
        "role_type": "SUPER_ADMIN"
      }
    }
  },
  {
    "match": "permissions/all",
    "body": {
      "response_code": 0,
      "data": [
        {
          "code": "MONITOR",
          "position": 1,
          "children": [
            { "code": "MONITOR_ALERT", "position": 1, "children": [] }
          ]
        }
      ]
    }
  }
]
```

The field name (`response_code` / `code` / `data`) must match the A2 interceptor door; `permissions` must cover all leaf code in the tree.

---

## C. coverage mode: preload configuration

Edit `scripts/preload.js` top `CONFIG` objects or replace them with CDP before infusion:

```javascript
const CONFIG = {
  loginPathRe: /\/(login|signin)(\/|$|\?)/i,
  mockTier: 'L1+L2',
  forward: true,
  recordDetail: true,
  extractUrlsFromResponse: true,
  neutralizeVueRouter: true,
  observe: { storageReads: false, cookieReads: false, xhrHeaders: true },
  neutralize: { fields: ['response_code', 'code'], success: 0 },
  stubs: [ /* Same. config.json stubs */ ],
  apiPattern: /\/(api|apis|v\d+|dev|internal|graphql)\//i,
};
```

Authentication: `window.__API_RECON_PRELOAD__ === true` and Pathname stabilized.

Export of recording results:

```javascript
JSON.stringify({
  apis: [...window.__API_RECON_LOG__],
  detail: window.__API_RECON_DETAIL__,
  routes: [...(window.__API_RECON_ROUTES__ || [])], // `__API_RECON_ROUTES__`
  observe: window.__API_RECON_OBSERVE__,
}, null, 2)
```

---

## D. preload / runtime Hook Capacity

Viewer built in preload (coverage) and runtime_harvest (depth):

| Hook Capacity | Value of discovery for API | Overwrite |
|---|---|---|
| Hook fetch / XHR.open | Application of requests URL/ method | ✅ `recordDetail` + `__API_RECON_LOG__` |
| Hook XHR.setRequestHeader | Find the Authorization header name | ✅ `observe.xhrHeaders` |
| Hook localStorage/cookie Read | Confirm the Session Key Name | ⚠️ Optional `observe.storageReads/cookieReads` |
| Vee Access Routes | Completion frontendRoutes | 1zz (carried route) |
| Vue Road Breaker by Guard Center / Login | Modular trigger, API. | `neutralizeVueRouter` + Original jump in transit and |
| React Retrieval Routes | Remedial route | Static + click; no specialized Hook |
| Page Jump Block (Link Path) | Leave Page Analysis | Just block login path and avoid operational navigation. |
| Hook Encryption Library (CryptoJS/SM et al.) | Encryption Arguments API body | Require manual Hook encryption function to enter; concluding writing config |
| Counter debugging bypass | Otherwise, runtime can't record API. | Must be handled manually; static is still available |

---

## E. Endpoint extraction norm (when there is too little static)

`harvest_static.py` relaxed in `extract_endpoints`, or manually:

```bash
grep -rhoaE '"/[a-z][A-Za-z0-9_/\-]{3,}"' js | sort -u
grep -rhoaE '/api/[a-zA-Z0-9_./-]+' js | sort -u
```

---

## F. Decomposition

| Symptom | Reasons |
|---|---|
| Static, API, very little. | Endpoint dialects do not match |
| hunk count | CSS-only or non-deployment chunk; 404 retryed |
| Runtime still displays login pages | Rendering gate error Review A1: Keyname, Container, Encoding, Domain |
| shell but module blank | Content door forge menu (A3); `routes` path may not be right |
| There's only bootstrap/locale on every route. | I section permission tree restoration; checking `role_permissions` + `permissions/all` double stub |
| Sidebar with an item but a subpage blank | Tree missing intermediate node or code inconsistent with `userRouteAuth` |
| API every login | Interceptor door & Confirm `neutralize`; nested fields need to expand walk logic |
| WS Frame is 0 | Users need to interact before subscribe; length `perRouteMs` |
| Response Blank | Only `forward: true` has a real response. |
| Missing Chromium | Install chromium or set `config.chromium`/ `CHROMIUM` |
| Mock, a lot of them are still login. | Hook `location.href` second = document-start + preload |
| List is empty | L3 empty array is normal; continuation point Tab/ settings/details |
| Mistake Redux action as a route | Filter internal path with get/set/change/clear/toggle/upload |
| Vue Still Jumping Login | preload not document-start — re-injection; or `neutralizeVueRouter: false` when manual guards are cleared |
| The response was URL but did not enter log | Open `extractUrlsFromResponse`; or extract manually from `__API_RECON_DETAIL__` |
| Unknown Authorization  header name | Open `observe.xhrHeaders` or DevTools to view request headers |
| Runtime Very Slow / Timeout | Change `waitUntil: domcontentloaded`; drop `routeTimeout`; do not use `networkidle2` |
| Proxy connection failed | Check the `proxy` environment variable; Puppeteer is consistent with curl proxy port |

---

## G. hardened Objective

Runtime is stuck at the shell when the session is gradually verified by the service end (a menu that does not allow forge to sign cookies, the service end to render unstub). Expected behavior:

- **Static enough to do endpoint enumerates**-- module path in code
- If authorized, run the same **real session**harness: `forward: true`, no neutralize, capture the real methods/params/responses

---

## H. List of single missions

1. Confirmation of the scope of the mandate
2. **Read**`scripts/harvest_static.py` * Adjusted to Target **Run `api_static.txt`, `routes.txt`
3. **Phase 1b**: Path anchoring window + binding layer `param_candidates.json` (section J)
4. Reverse A1/A2/A3 Write site exclusive `config.json`
5. **Read and adjust**`runtime_harvest.js`/ `preload.js` and then execute
6. `runtimeMode=depth`:`npm install`
7. `runtimeMode=coverage/both`: documentation-start injected preload browser MCP dynamic emulation +**parameter trigger matrix**
**I section permission tree restoration**
9. Multi-Specific Diff + Error Invert `params_merged.json`
10. Merge ~ `site_map.json` + `api_merged.txt`, honest mark overlay, vulnerabilities and script change points

---

## I. Rights Tree Restoration (Phase 4 Deepening)

Use forge when simple `menus: [{ path, show: true }]` is not valid and the submodule is still non-mount.

### I1. Positioning auth module

```bash
grep -l 'userRouteAuth' js/*.js
grep -l 'routeMap\|routeLink' js/*.js
grep -rhoaE 'getResultTree|role_permissions|permissions/all' js | head
```

Records:**Permission API path**,**Response field name**,**Consumption chunk filename**.

### I2. Extract routeMap

```bash
python3 scripts/extract_route_map.py recon/js recon/
# Outputs recon/route_map.json
```

If `[!] no routeMap pattern found`: relaxed `extract_route_map.py`, or handmade grep:

```bash
grep -rhoaE '([A-Z_][A-Z0-9_]*):\{name:"[^"]*",link:"/[^"]+"\}' js | head -20
```

### I3. Build permission tree + stub

```bash
python3 scripts/build_perm_tree.py recon/js recon/ --config recon/config.json  # `--config` uses `config.json`
```

Script logic:
1. Parsing `userRouteAuth={MONITOR:{url:...},...}` (with webpack alias `He=o.DASHBOARD`)
2. Resolve the real path with `route_map.json`
3. Extrapolation by code prefix (`MONITOR_ALERT` `MONITOR`)
4. Output `permissions_tree.json`、`permissions_all_stub.json`、`role_permissions_stub.json`
5. Automatically write `stubs` with extension `routes` when `--config`

**Adjusted to target**(at the top of the script):
- `DEFAULT_ROOTS`: Top Module Code List
- `DEFAULT_PREFIX_PARENT`:`PREFIX_` *parent map
- `DEFAULT_EXTRA_PARENT`: Non-prefixed orphan node

### I4. Verify stub consistency

```bash
# permissions Quantities should ≈ userRouteAuth Number of entries
wc -l recon/perm_codes_all.txt
# routes Should be overwritten route_map All link
python3 -c "import json; m=json.load(open('recon/route_map.json')); r=set(json.load(open('recon/config.json'))['routes']); print('missing', [v['link'] for v in m.values() if v['link'] not in r])"
```

### I5. Run runtime and compare

```bash
node recon/runtime_harvest.js recon/config.json
# Contrast Forge Front and read runtime_api.json Number of articals; Chuck/attracking for modules to appear API
```

| Forge before | Forge after (successful) |
|---|---|
| Same route, three to five bootstraps. | Different route triggers |
| `/api/locale/language` only | Show `/api/web/...` module endpoint |
| `routes.txt` bits of route | `routes` 80–110+ from route_map |

### I6. When it still fails

- **coverage mode**: Click on sidebar + Tab, permissions gating may request only after interactive
- **stub field**: contrast API (curl + true session) with stub viewing
- **Additional guards**: button level check with grep `hasPermission|checkRole|func.`, extension `role_permissions.permissions`
- **Static spinster**: Module API path is still `api_static.txt`, rantime only METHOD/body; parameter `param_candidates.json`+ recorded sample

---

## J. Parameter reverse (Phase 1b/ 5b/5c)

**Methodology, non-universal script.**Find path with regular rules; find parameters to expand windows with anchor + UI chain + multiple samples diff + error reverse.

### J1. An anchor extension window - find a package object from a path

```bash
# By Phase 1 Known path To anchor
grep -n '"/api/user/list"' js/*.js
grep -rhoaE '.{0,120}("/api[^"]+").{0,200}' js | head
grep -rhoaE '(params|data|body|payload)\s*:\s*\{' js | head
grep -rhoaE '(get|post|put|delete|patch)\([^,]+,\s*\{' js | head
```

### J2. Packaging and transmission patterns

```bash
# axios / Uniform request
grep -rhoaE '(axios|request)\.(get|post|put|delete|patch)\(' js | head
grep -rhoaE 'interceptors\.(request|response)' js | head

# GraphQL
grep -rhoaE '(query|mutation)\s+\w+|gql`|graphql\(' js | head
grep -rhoaE '\$[a-zA-Z_]+\s*:\s*(Int|String|Boolean|\[)' js | head

# FormData / multipart
grep -rhoaE 'FormData|\.append\(' js | head

# Path Parameters
grep -rhoaE 'path:\s*"/[^"]*:[^"]+"' js | head
grep -rhoaE 'useParams|route\.params|\$route\.params' js | head
```

### J3. Validation door - must be filled / formatted / enumeration

```bash
grep -rhoaE '(required|message|pattern|enum|validator)\s*:' js | head
grep -rhoaE 'yup\.|zod\.|async-validator|Form\.Item|a-form-item|el-form-item' js | head
grep -rhoaE 'rules\s*:\s*\[|name:\s*["\'][a-zA-Z_]+["\']' js | head
grep -rhoaE 'label.*value|options\s*:\s*\[' js | head
```

### J4. Binding layers - Form API

```bash
grep -rhoaE 'onFinish|handleSubmit|getFieldsValue|validateFields' js | head
grep -rhoaE '(pick|omit|transform|dayjs|moment)\(' js | head
```

Runtime Supplement: DevTools Network Request **Launcher**(call start) from `fetch`/`send` up to the package function.

### J5. Encryption Parameters

```bash
grep -rhoaE 'encrypt|decrypt|sign|CryptoJS|sm2|sm3|sm4|RSA|AES' js | head
```

**Do not guess fields**— Hook Encryption function **Enter**, pre-encrypt payload; concluding `config.json`/ `param_candidates.json`.

### J6. Parameters trigger matrix (Phase 3 will do)

Record each module by operation, diff request body/query:

| Operation | Attention |
|---|---|
| List First Screen | Page Break Defaults |
| Search | keyword、filters |
| Advanced Filter | optional Fields |
| New/edit | Fullity |
| Bulk/Export | `ids[]`、`exportType` |
| Sorting and pagination | `sortField`、`order` |

Outputs `param_samples.json`:`[{ "path", "method", "action": "search", "body", "query", "headers" }]`

### J7. Confidence Rule

| Confidence | Condition |
|---|---|
| **High** | Static callsite + rantime ~2 identical samples |
| **Medium** | Static only, or only 1 runtime |
| **Low** | Response/Error invert, not twice verified |
| **To be triggered** | Static known fields, UI/ clearance not available |

### J8. It's a perfect scene.

| Scenario | Order |
|---|---|
| REST List Page | J1 group objects J6 diff J3 riles |
| New/edit forms | J3 Form name J4 chain rantime submission + intentionally left blank 400 |
| GraphQL | J2 variables declaration rantime operations per variables |
| Encryption body | J5 Hook Enter * Pre-encryption field is real params |

### J9. Map with api-recon phase

| api-recon | Parameter recon |
|---|---|
| Phase 1 Static | J1 Anchor-Based Window Expansion |
| Phase 2A2 Interceptor | Global injection fields (tenantId, sign) |
| Phase 3 runtime | J6 Trigger Matrix + `param_samples.json` |
| Phase 4 Permission Tree | Different module forms Permissions sufficient to trigger the full field |
| Phase 5 Merge | `params_merged.json` + confidence; not-for-one samples are required |

### J10. Decomposition

| Symptom | Processing |
|---|---|
| Static has field names. Runtime never appeared. | Mark 'to trigger '; supplement permissions tree/ point advanced filter / connect each option |
| Different body shape from path | Normal - press `action` sub-record and do not force merge schema |
| stub respond but want to see | **See outbound request**body/headers, do not respond to rebuttal from stub |
| 400 newspapers | Note outer packaging `data`/`bizData`/`variables` |
| GraphQL see only opation name | Expand `variables` JSON; static search `$var: Type` |

---
