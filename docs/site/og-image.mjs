// Makes the website's link-preview image, docs/site/assets/og-image.jpg (1200 × 630, as social
// sites show it): the logo, the name and a line on what ClonoLoom is, over the Repertoire view's
// screenshot in the dark theme (docs/images/diversity-dark.webp), drawn in headless Chrome.
//
//   node docs/site/og-image.mjs
//
// Needs Chrome, Chromium, Edge or Brave (CHROME chooses). Run it again when the screenshot changes.

import { readFileSync, rmSync, writeFileSync, mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { launch } from '../../validation/cdp.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const repo = resolve(here, '../..');
const dataURL = (file, type) => `data:${type};base64,${readFileSync(join(repo, file)).toString('base64')}`;

const page = `<!doctype html>
<html><head><meta charset="utf-8"><style>
  html, body { margin: 0; width: 1200px; height: 630px; overflow: hidden; }
  body { background: radial-gradient(900px 500px at 15% 0%, rgba(122, 92, 255, 0.35), transparent 60%), radial-gradient(700px 400px at 100% 100%, rgba(200, 64, 111, 0.28), transparent 60%), #07090e;
    font-family: Inter, ui-sans-serif, system-ui, -apple-system, "Segoe UI", sans-serif; color: #eef0f6; }
  .text { position: absolute; left: 64px; top: 64px; width: 470px; }
  .brand { display: flex; align-items: center; gap: 16px; font-size: 44px; letter-spacing: -0.02em; }
  .brand img { width: 64px; height: 64px; border-radius: 14px; }
  .brand b { background: linear-gradient(135deg, #f0567f, #f2b04a); -webkit-background-clip: text; background-clip: text; color: transparent; }
  h1 { font-size: 40px; line-height: 1.15; letter-spacing: -0.02em; margin: 40px 0 20px; }
  p { font-size: 22px; line-height: 1.45; color: rgba(238, 240, 246, 0.78); margin: 0; }
  .shot { position: absolute; left: 580px; top: 70px; width: 820px; border-radius: 12px; overflow: hidden;
    box-shadow: 0 30px 80px rgba(0, 0, 0, 0.6); border: 1px solid rgba(238, 240, 246, 0.14); }
  .shot img { display: block; width: 100%; }
</style></head><body>
  <div class="text">
    <div class="brand"><img src="${dataURL('web/favicon.svg', 'image/svg+xml')}" alt=""><span>Clono<b>Loom</b></span></div>
    <h1>Immune repertoires you can check</h1>
    <p>T- and B-cell receptor data checked, clonotypes defined explicitly, diversity and overlap measured with their uncertainty, on your own computer.</p>
  </div>
  <div class="shot"><img src="${dataURL('docs/images/diversity-dark.webp', 'image/webp')}" alt=""></div>
</body></html>`;

const dir = mkdtempSync(join(tmpdir(), 'clonoloom-og-'));
const file = join(dir, 'card.html');
writeFileSync(file, page);
const b = await launch({ width: 1200, height: 630 });
try {
  await b.goto(pathToFileURL(file).href, 600);
  await b.capture(join(here, 'assets/og-image.jpg'), { format: 'jpeg', quality: 88 });
} finally {
  await b.close();
  rmSync(dir, { recursive: true, force: true });
}
console.log('Wrote docs/site/assets/og-image.jpg');
