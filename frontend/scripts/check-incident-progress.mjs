// Run Vite, then set PLAYWRIGHT_MODULE and optionally BROWSER_EXECUTABLE / UI_BASE_URL.
import assert from 'node:assert/strict';
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
const browser = await chromium.launch({ executablePath: process.env.BROWSER_EXECUTABLE });
const page = await browser.newPage();
const incident = {
  incidentId: 'progress-check', alert: 'KubePodCrashLooping', namespace: 'test',
  workload: 'python-syntax-crash', severity: 'warning', state: 'EvidenceCollected',
  createdAt: new Date().toISOString(), evidence: { metrics: [], logs: [], events: [] },
};
let visible = false;
const errors = [];
page.on('pageerror', e => errors.push(e.message));
await page.route(url => url.pathname.startsWith('/api/'), async route => {
  const path = new URL(route.request().url()).pathname;
  const body = path === '/api/incidents/counts' ? { open: +visible, acknowledged: 0, all: +visible }
    : path === '/api/incidents' ? (visible ? [incident] : [])
    : path.endsWith('/reanalyze') ? { state: 'idle', elapsedSec: 0 } : incident;
  await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
});
try {
  await page.goto(`${process.env.UI_BASE_URL || 'http://127.0.0.1:5173'}/incidents`);
  await page.waitForFunction(() => document.body.innerText.includes('표시할 인시던트가 없습니다.'));
  visible = true;
  await page.waitForFunction(() => document.body.innerText.includes('progress-check'), undefined, { timeout: 12000 });
  assert.equal(await page.evaluate(() => document.querySelector('.seg-count').textContent), '1');
  await page.click('.incident-link');
  await page.waitForFunction(() => [...document.querySelectorAll('button')].some(b => b.disabled && b.textContent.includes('AI 분석 중')));
  incident.state = 'DiagnosisCompleted';
  incident.diagnosis = { rootCause: "SyntaxError: expected ':'", summary: '함수 정의에 콜론을 추가하세요.', confidence: .97, proposedActions: [] };
  await page.waitForFunction(() => document.body.innerText.includes("SyntaxError: expected ':'"), undefined, { timeout: 12000 });
  assert.deepEqual(errors, []);
  console.log('PASS: new incidents and completed diagnoses appear without reloading; pending analysis cannot be restarted.');
} finally {
  await browser.close();
}
