import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import ts from 'typescript';

const source = await readFile(new URL('../src/lib/format.ts', import.meta.url), 'utf8');
const js = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext } }).outputText;
const format = await import(`data:text/javascript;base64,${Buffer.from(js).toString('base64')}`);

assert.deepEqual(
  format.splitNarrative?.('첫 원인이다. 재시작은 5회다.\n\n수정한다.'),
  ['첫 원인이다.', '재시작은 5회다.', '수정한다.'],
);
assert.deepEqual(
  format.splitNarrative?.('python:3.12-slim 이미지와 spec.template 값은 그대로 둔다.'),
  ['python:3.12-slim 이미지와 spec.template 값은 그대로 둔다.'],
);
assert.deepEqual(format.splitNarrative?.('  단일 문장  '), ['단일 문장']);
console.log('PASS: AI narrative paragraphs preserve text and technical identifiers');
