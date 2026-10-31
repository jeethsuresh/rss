import assert from 'node:assert/strict';
import { mkdtemp, readdir, rm } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { readFile } from 'node:fs/promises';
import { spawn } from 'node:child_process';
import { createServer } from 'node:net';
const config = JSON.parse(await readFile('desktop-release.config.json', 'utf8'));
const { chromium } = await import(config.slug === 'office' ? '@playwright/test' : 'playwright');
const profile = await mkdtemp(path.join(os.tmpdir(), config.slug + '-native-'));
let executable = path.resolve(process.argv[2]);
if (process.platform === 'darwin') {
  const directory = path.join(executable, 'Contents/MacOS');
  const files = await readdir(directory);
  assert.equal(files.length, 1);
  executable = path.join(directory, files[0]);
}
const launch = async () => {
  const reservation = createServer();
  await new Promise(resolve => reservation.listen(0, '127.0.0.1', resolve));
  const port = reservation.address().port;
  await new Promise(resolve => reservation.close(resolve));
  const env = { ...process.env, OFFICE_DATA: path.join(profile, 'workspace'),
    OFFICE_PROFILE: path.join(profile, 'profile'), RSS_PROFILE: path.join(profile, 'profile'),
    ELECTRON_ENABLE_LOGGING: '1' };
  delete env.ELECTRON_RUN_AS_NODE;
  delete env.NODE_OPTIONS;
  const child = spawn(executable, [`--remote-debugging-port=${port}`, '--remote-debugging-address=127.0.0.1',
    ...(process.platform === 'linux' ? ['--no-sandbox', '--disable-dev-shm-usage'] : [])], {env, stdio: ['ignore', 'pipe', 'pipe']});
  let logs = '';
  for (const stream of [child.stdout, child.stderr]) stream.on('data', data => { logs = (logs + data).slice(-16000); });
  let launchError;
  child.once('error', error => { launchError = error; });
  let browser;
  try {
    for (let attempt = 0; attempt < 120; attempt++) {
      if (launchError || child.exitCode !== null) throw launchError ?? new Error('Native process exited: ' + logs);
      try {
        const response = await fetch(`http://127.0.0.1:${port}/json/version`);
        if (response.ok) { browser = await chromium.connectOverCDP(`http://127.0.0.1:${port}`); break; }
      } catch {}
      await new Promise(resolve => setTimeout(resolve, 250));
    }
    if (!browser) throw new Error('Native browser did not start: ' + logs);
    return {
      firstWindow: async () => {
        const context = browser.contexts()[0];
        return context.pages()[0] ?? await context.waitForEvent('page', {timeout: 30000});
      },
      close: async () => {
        // Electron may close the CDP transport before acknowledging Browser.close.
        // Keep an explicit deadline alive and verify the real process has exited.
        const deadline = async operation => {
          let timer;
          try { await Promise.race([operation, new Promise(resolve => { timer = setTimeout(resolve, 2000); })]); }
          finally { clearTimeout(timer); }
        };
        await deadline((async () => {
          const cdp = await browser.newBrowserCDPSession();
          await cdp.send('Browser.close');
        })().catch(() => {}));
        for (let attempt = 0; attempt < 100 && child.exitCode === null && child.signalCode === null; attempt++) {
          await new Promise(resolve => setTimeout(resolve, 100));
        }
        if (child.exitCode === null && child.signalCode === null) {
          child.kill();
          throw new Error('Native application did not exit cleanly: ' + logs);
        }
        await deadline(browser.close().catch(() => {}));
      },
    };
  } catch (error) {
    child.kill();
    throw error;
  }
};
let app;
try {
  app = await launch();
  let page = await app.firstWindow();
  page.on('pageerror', error => console.error('Native renderer:', error));
  assert.equal(await page.evaluate(() => typeof require), 'undefined', 'renderer has no Node integration');
  if (config.slug === 'office') {
    await page.getByText('All changes saved', { exact: true }).waitFor();
    const note = page.getByLabel('Document text', { exact: true }).first();
    await note.fill('This native workspace survives a Store update.');
    await page.getByText('All changes saved', { exact: true }).waitFor();
    // Wait behind IndexedDB and Go journal writes before a clean shutdown.
    await page.waitForTimeout(750);
  } else {
    await page.getByRole('tab', { name: /^RSS Reader/ }).waitFor();
    await page.waitForFunction(() => !!window.rss);
    const folders = await page.evaluate(async () => {
      await window.rss.folders.create('Native desktop smoke');
      return window.rss.folders.list();
    });
    assert.ok(folders.some(folder => folder.name === 'Native desktop smoke'));
    assert.ok((await page.locator('body').innerText()).trim().length > 20, 'native reader rendered');
  }
  await app.close(); app = undefined;
  // OS process exit and sidecar listener cleanup may arrive in separate events.
  for (let attempt = 0; attempt < 20; attempt++) {
    try {
      app = await launch();
      page = await app.firstWindow();
      if (config.slug === 'office') {
        await page.getByText('All changes saved', { exact: true }).waitFor({ timeout: 10000 });
        assert.ok((await page.getByLabel('Document text', { exact: true }).first().innerText()).includes('This native workspace survives a Store update.'));
      } else {
        await page.waitForFunction(() => !!window.rss);
        const folders = await page.evaluate(() => window.rss.folders.list());
        assert.ok(folders.some(folder => folder.name === 'Native desktop smoke'));
      }
      break;
    } catch (error) {
      await app?.close().catch(() => {}); app = undefined;
      if (attempt === 19) throw error;
      await new Promise(resolve => setTimeout(resolve, 250));
    }
  }
  console.log(config.name + ': packaged native UI, local backend and persistence across relaunch passed');
} catch (error) {
  console.error(error);
  throw error;
} finally {
  await app?.close().catch(error => { console.error(error); process.exitCode = 1; });
  await rm(profile, { recursive: true, force: true, maxRetries: 20, retryDelay: 100 });
}
