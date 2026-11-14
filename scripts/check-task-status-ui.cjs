// Read-only browser regressions against a STATIC_UI_URL serving web/out.
// Requires Playwright via PLAYWRIGHT_PATH (module path) or normal resolution.
// Backend execution/control handlers are intentionally not exercised: every API
// response is synthetic, and non-GET requests and external origins are rejected.
const { chromium } = require(process.env.PLAYWRIGHT_PATH || 'playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const base = process.env.STATIC_UI_URL;
assert(base && ['127.0.0.1', 'localhost'].includes(new URL(base).hostname), 'Use a loopback static server');
const results = [];
const timestamp = 1794693626;
const rerunLabel = 'Rerun (resets run clock)';
const inactiveLabel = 'No recent recorded activity';
const deferred = () => {
  let resolve;
  const promise = new Promise(done => { resolve = done; });
  return { promise, resolve };
};

function task(id, state) {
  return {
    id, name: `Status fixture ${id}`, description: `Status fixture ${id}`,
    goal: 'Offline display verification', exploration_id: Number(id),
    status: state.lifecycle || (state.mode === 'paused' ? 'paused' : 'running'),
    paused: state.mode === 'paused', created_at: '2026-11-14T21:04:59Z',
    created_unix: 1794690299, last_activity_unix: state.timestamp ?? timestamp,
    completed_at: '', completed_unix: 0, timeout_seconds: 600,
    goals_total: 2, goals_met: 0,
    tokens: { input_tokens: 0, output_tokens: 0, cache_read_tokens: 0, cache_write_tokens: 0 },
  };
}

async function fixture(browser, initial = {}) {
  const state = { mode: 'stalled', ...initial };
  const context = await browser.newContext({ viewport: { width: 1600, height: 1050 }, locale: 'en-US', timezoneId: 'UTC' });
  await context.addInitScript(() => {
    localStorage.setItem('antifox_token', 'offline-fixture');
    localStorage.setItem('antifox-lang', 'en-US');
  });
  const calls = [], writes = [], external = [], errors = [];
  await context.route('**/*', async route => {
    const request = route.request();
    const url = new URL(request.url());
    if (url.origin !== base) {
      external.push(request.url());
      return route.abort();
    }
    if (!url.pathname.startsWith('/api/')) return route.continue();
    calls.push({ path: url.pathname, task: url.searchParams.get('task'), method: request.method() });
    if (request.method() !== 'GET') {
      writes.push(url.pathname);
      return route.fulfill({ status: 405, json: { error: 'Read-only fixture' } });
    }
    let body = {};
    if (url.pathname === '/api/tasks') body = { tasks: ['5', '6'].map(id => task(id, state)), active: '5' };
    else if (url.pathname === '/api/stats') {
      const id = url.searchParams.get('task') || '5';
      body = {
        engine_mode: state.mode, llm_configured: state.llm ?? true, assets: 0,
        active_task: {
          id: state.wrongId ? 'other-task' : id, description: `Status fixture ${id}`, goal: 'Offline display verification',
          running: state.mode !== 'paused', paused: state.mode === 'paused', in_flight: 0,
          last_activity: state.timestamp ?? timestamp, stalled: state.mode === 'stalled', goals_total: 2, goals_met: 0,
        }, exploration: {},
      };
      if (state.hold && state.hold.id === id) {
        const hold = state.hold;
        state.hold = null;
        hold.entered.resolve();
        await hold.release.promise;
        await route.fulfill({ json: body }).catch(() => {}); // May already be aborted by the UI.
        hold.completed.resolve();
        return;
      }
      if (state.fail) return route.fulfill({ status: 503, json: { error: 'Synthetic status outage' } });
    }
    else if (url.pathname.startsWith('/api/intercept/')) body = { items: [] };
    else if (url.pathname === '/api/llm/profiles') body = { profiles: [] };
    else if (url.pathname === '/api/rbac/me') body = { permissions: [], user: { id: 1, username: 'fixture' }, roles: [] };
    else if (url.pathname === '/api/conversations') body = { conversations: [] };
    else if (url.pathname.startsWith('/api/exploration/')) body = [];
    return route.fulfill({ json: body });
  });
  const page = await context.newPage();
  page.on('pageerror', e => errors.push(e.message));
  page.setDefaultTimeout(15000);
  const header = () => page.locator('header').filter({ has: page.getByRole('heading', { name: /^Status fixture/ }) });
  const engine = () => page.getByText('Engine state', { exact: true }).locator('..');
  const recent = () => page.getByText('Recent activity', { exact: true }).locator('..');
  const open = async () => {
    await page.goto(`${base}/function/tasks/detail/?id=5`, { waitUntil: 'domcontentloaded' });
    await page.getByRole('heading', { name: 'Status fixture 5', exact: true }).waitFor();
    await page.getByRole('tab', { name: 'Overview', exact: true }).click();
  };
  const agree = async label => {
    await header().getByText(label, { exact: true }).waitFor();
    await engine().getByText(label, { exact: true }).waitFor();
  };
  const close = async name => {
    assert.deepEqual(writes, [], 'No API mutation requests');
    assert.deepEqual(external, [], 'No external-origin requests');
    assert.deepEqual(errors, [], 'No JavaScript errors');
    const result = { name, result: 'PASS', writeRequests: writes, externalRequests: external, javascriptErrors: errors,
      header: await header().count() ? await header().innerText() : null,
      engine: await engine().count() ? await engine().innerText() : null, apiCalls: calls.length };
    if (process.env.STATUS_EVIDENCE) {
      await page.screenshot({ path: path.join(process.env.STATUS_EVIDENCE, `${name}.png`), fullPage: true });
    }
    results.push(result);
    console.log(`PASS ${name}`);
    await context.close();
  };
  return { state, context, page, header, engine, recent, open, agree, close, calls };
}

(async () => {
  const browser = await chromium.launch({ headless: true,
    ...(process.env.CHROMIUM_PATH ? { executablePath: process.env.CHROMIUM_PATH } : {}) });
  try {
    // Regression 1 + 3: real-shaped JSON previously produced Idle/Stalled and a blank time.
    let f = await fixture(browser);
    await f.open();
    await f.agree(inactiveLabel);
    const expectedTime = await f.page.evaluate(ts => new Date(ts * 1000).toLocaleString('en-US'), timestamp);
    assert((await f.recent().innerText()).includes(expectedTime));
    assert.equal(await f.page.getByRole('button', { name: rerunLabel, exact: true }).count(), 0);
    await f.close('mapping-and-unix-timestamp');

    // Regression 2: the header previously stayed Paused while Overview updated.
    f = await fixture(browser, { mode: 'paused' });
    await f.open();
    await f.agree('Paused');
    f.state.mode = 'exploring';
    await f.agree('Exploring');
    assert.equal(await f.page.getByRole('button', { name: 'Resume', exact: true }).count(), 0);
    await f.page.getByRole('button', { name: 'Pause', exact: true }).waitFor();
    await f.close('shared-refresh');

    f = await fixture(browser, { mode: 'exploring' });
    await f.open();
    f.state.fail = true;
    await f.header().getByText(/^Status is stale/).waitFor();
    assert.equal(await f.page.getByText(/^Status is stale/).count(), 2);
    assert(await f.page.getByRole('button', { name: 'Pause', exact: true }).isDisabled());
    await f.close('outage-retains-marked-stale-data');

    f = await fixture(browser, { mode: 'exploring' });
    await f.open();
    f.state.fail = true;
    await f.header().getByText(/^Status is stale/).waitFor();
    f.state.fail = false;
    f.state.mode = 'paused';
    await f.agree('Paused');
    assert.equal(await f.page.getByText(/^Status is stale/).count(), 0);
    assert(await f.page.getByRole('button', { name: 'Resume', exact: true }).isEnabled());
    await f.close('observation-recovery');

    for (const initial of [{ fail: true }, { wrongId: true }]) {
      f = await fixture(browser, initial);
      await f.page.goto(`${base}/function/tasks/detail/?id=5`);
      await f.page.getByText('Status unavailable', { exact: true }).waitFor();
      assert.equal(await f.page.getByText('Task 5 not found', { exact: true }).count(), 0);
      await f.close(initial.fail ? 'initial-outage' : 'reject-wrong-task-observation');
    }

    for (const [lifecycle, label] of [['done', 'Done'], ['failed', 'Failed'], ['timeout', 'Timed Out']]) {
      f = await fixture(browser, { lifecycle });
      await f.open();
      await f.agree(label);
      await f.page.getByRole('button', { name: rerunLabel, exact: true }).waitFor();
      assert.equal(await f.page.getByRole('button', { name: 'Pause', exact: true }).count(), 0);
      await f.close(`terminal-${lifecycle}`);
    }

    f = await fixture(browser, { timestamp: 0, llm: false });
    await f.open();
    assert((await f.recent().innerText()).includes('—'));
    assert.equal(await f.header().getByText('LLM configured', { exact: true }).count(), 0);
    await f.close('missing-time-and-unconfigured-provider');

    // A late response from a previous task must not populate the new task's view.
    const hold = { id: '5', entered: deferred(), release: deferred(), completed: deferred() };
    f = await fixture(browser, { mode: 'paused', hold });
    await f.page.goto(`${base}/function/tasks/detail/?id=5`, { waitUntil: 'domcontentloaded' });
    await hold.entered.promise;
    f.state.mode = 'exploring';
    await f.page.evaluate(() => window.history.pushState({}, '', '?id=6'));
    await f.page.getByRole('heading', { name: 'Status fixture 6', exact: true }).waitFor();
    await f.page.getByRole('tab', { name: 'Overview', exact: true }).click();
    await f.agree('Exploring');
    hold.release.resolve();
    await hold.completed.promise;
    await f.page.evaluate(() => new Promise(done => requestAnimationFrame(() => requestAnimationFrame(done))));
    assert.equal(await f.page.getByRole('heading', { name: 'Status fixture 5', exact: true }).count(), 0);
    await f.agree('Exploring');
    await f.close('late-response-after-task-switch');

    f = await fixture(browser);
    await f.page.goto(`${base}/function/tasks/`);
    const row = f.page.getByRole('row').filter({ hasText: 'Status fixture 5' });
    await row.getByRole('button', { name: 'Actions', exact: true }).click();
    assert.equal(await f.page.getByRole('menuitem', { name: rerunLabel, exact: true }).count(), 0);
    await f.close('task-list-no-inactivity-rerun');

    const summary = { result: 'PASS', scenarios: results.length, fixtureOnly: true, results };
    if (process.env.STATUS_EVIDENCE) fs.writeFileSync(path.join(process.env.STATUS_EVIDENCE, 'browser-results.json'), JSON.stringify(summary, null, 2));
    console.log(JSON.stringify({ result: 'PASS', scenarios: results.length, writeRequests: 0, externalRequests: 0, javascriptErrors: 0 }));
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
