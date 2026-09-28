// Run Vite first, then: PLAYWRIGHT_MODULE=/path/to/playwright/index.mjs node scripts/check-ui.mjs
// Playwright is a local verification tool; it is not a production dependency.
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import ts from 'typescript';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
const source = await readFile(new URL('../src/api/mock.ts', import.meta.url), 'utf8');
const fixtures = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext } }).outputText;
const { mockIncidents, mockSettings, mockPolicies } = await import(`data:text/javascript;base64,${Buffer.from(fixtures).toString('base64')}`);
const browser = await chromium.launch();
const page = await browser.newPage();
const errors = [];
page.on('pageerror', error => errors.push(error.message));
const acknowledged = new Set();
let settings = structuredClone(mockSettings);
let rules = [];
let saved = false;
await page.route(url => url.pathname.startsWith('/api/'), async route => {
  const req = route.request();
  const url = new URL(req.url());
  const path = url.pathname.replace('/api', '');
  const method = req.method();
  let body;
  if (path === '/incidents/counts') body = { open: mockIncidents.length - acknowledged.size, acknowledged: acknowledged.size, all: mockIncidents.length };
  else if (path === '/incidents') body = mockIncidents.filter(i => url.searchParams.get('filter') === 'all' || acknowledged.has(i.incidentId) === (url.searchParams.get('filter') === 'acknowledged'));
  else if (path.endsWith('/reanalyze')) body = { state: 'idle', elapsedSec: 0 };
  else if (path.startsWith('/incidents/')) {
    const id = decodeURIComponent(path.split('/')[2]);
    if (method === 'PATCH') {
      if (req.postDataJSON().acknowledged) acknowledged.add(id); else acknowledged.delete(id);
      return route.fulfill({ status: 204 });
    }
    body = mockIncidents.find(i => i.incidentId === id);
  } else if (path === '/settings') {
    if (method === 'PUT') { settings = req.postDataJSON(); saved = true; }
    body = settings;
  } else if (path === '/secrets') body = { aiApiKey: false, gitToken: false };
  else if (path === '/ai/status') body = { endpoint: 'http://test.invalid/v1', model: 'test-model', providerKind: 'local', providerName: 'Test' };
  else if (path === '/policies') body = mockPolicies;
  else if (path === '/ignores') {
    if (method === 'POST') rules.push({ id: 1, keyword: req.postDataJSON().keyword, enabled: true, createdAt: new Date().toISOString() });
    body = method === 'GET' ? { rules, config: ['KubeCPUOvercommit'] } : rules.at(-1);
  } else if (path === '/ignores/1') {
    if (method === 'PATCH') rules[0].enabled = req.postDataJSON().enabled;
    if (method === 'DELETE') rules = [];
    return route.fulfill({ status: 204 });
  } else throw new Error(`Unhandled API request: ${method} ${path}`);
  await route.fulfill({ json: body });
});
const base = process.env.UI_BASE_URL || 'http://127.0.0.1:5173';
const routes = ['/', '/incidents', `/incidents/${mockIncidents[0].incidentId}`, '/approvals', '/policies', '/ignores', '/settings'];
try {
  if (!process.argv.includes('--interactions-only')) {
  for (const theme of ['dark', 'light']) {
    await page.goto(base);
    await page.evaluate(t => localStorage.setItem('kubesentinel-theme', t), theme);
    for (const width of [1440, 768, 390]) {
      await page.setViewportSize({ width, height: 1000 });
      for (const route of routes) {
        await page.goto(base + route);
        await page.waitForSelector('h1');
        await page.locator('.skeleton').waitFor({ state: 'hidden' });
        assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true, `${theme} ${width}px ${route}: page overflow`);
        assert.equal(await page.locator('nav a').count(), 6);
        assert.equal(await page.locator('nav a.active').count(), 1);
      }
    }
  }
  console.log('PASS: 7 routes × 3 viewport sizes × 2 themes; no page overflow');
  }
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto(base);
  await page.locator('.incident-link').first().focus();
  await page.keyboard.press('Enter');
  await page.waitForURL('**/incidents/' + mockIncidents[0].incidentId);
  await page.getByRole('heading', { name: 'AI 진단 (RCA)' }).waitFor();
  await page.goBack();
  assert.equal(new URL(page.url()).pathname, '/');
  // Headless Chromium does not consistently create a tab for Ctrl-click.
  // The link must preserve the browser default and leave the current route alone.
  const defaultAllowed = await page.locator('.incident-link').first().evaluate(link =>
    link.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, ctrlKey: true })));
  assert.equal(defaultAllowed, true);
  assert.equal(new URL(page.url()).pathname, '/');
  await page.goto(base + '/incidents');
  await page.getByLabel('인시던트 검색').fill('search-indexer');
  assert.equal(await page.locator('tbody tr').count(), 1);
  await page.getByLabel('인시던트 검색').fill('not-an-incident');
  await page.getByText('검색 조건에 맞는 인시던트가 없습니다.').waitFor();
  await page.getByLabel('인시던트 검색').fill('');
  await page.locator('tbody input[type=checkbox]').first().click();
  await page.waitForFunction(() => document.querySelectorAll('tbody tr').length === 3);
  assert.equal(acknowledged.size, 1);
  await page.getByRole('tab', { name: /확인됨/ }).click();
  await page.waitForFunction(() => document.querySelectorAll('tbody tr').length === 1);
  await page.getByRole('button', { name: '확인 취소' }).click();
  await page.getByText('확인됨 처리한 인시던트가 없습니다.').waitFor();
  await page.goto(base + '/ignores');
  await page.getByLabel('무시 키워드').fill('test-workload');
  await page.getByRole('button', { name: '추가', exact: true }).click();
  await page.getByRole('button', { name: '해제', exact: true }).click();
  await page.getByText('해제됨', { exact: true }).waitFor();
  await page.getByRole('button', { name: '활성화', exact: true }).click();
  await page.getByRole('button', { name: '삭제', exact: true }).click();
  await page.getByText('등록된 무시 규칙이 없습니다.').waitFor();
  await page.goto(base + '/settings');
  await page.getByRole('radio', { name: 'Light', exact: true }).click();
  await page.reload();
  assert.equal(await page.locator('html').getAttribute('data-theme'), 'light');
  await page.getByRole('radio', { name: 'Dark', exact: true }).click();
  assert.equal(await page.locator('html').getAttribute('data-theme'), 'dark');
  await page.getByRole('button', { name: '저장', exact: true }).click();
  await page.getByText('저장되었습니다 (DB)').waitFor();
  assert.equal(saved, true);
  assert.equal(errors.length, 0, errors.join('\n'));
  console.log('PASS: keyboard detail navigation, search/empty state, acknowledge/undo, ignore add/toggle/delete, theme persistence, settings save; no JS errors');
} finally {
  await browser.close();
}
