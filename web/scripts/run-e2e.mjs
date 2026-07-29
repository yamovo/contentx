import { spawn } from 'node:child_process'
import { existsSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import process from 'node:process'

const viteCli = fileURLToPath(new URL('../node_modules/vite/bin/vite.js', import.meta.url))
const playwrightCli = fileURLToPath(new URL('../node_modules/@playwright/test/cli.js', import.meta.url))
const serverURL = 'http://127.0.0.1:3000'

const server = spawn(
  process.execPath,
  [viteCli, '--host', '127.0.0.1'],
  { stdio: 'inherit', windowsHide: true },
)

async function waitForServer() {
  for (let attempt = 0; attempt < 120; attempt += 1) {
    if (server.exitCode !== null) {
      throw new Error(`Vite exited before becoming ready (code ${server.exitCode})`)
    }
    try {
      const response = await fetch(serverURL)
      if (response.ok) return
    } catch {
      // Server is still starting.
    }
    await new Promise((resolve) => setTimeout(resolve, 250))
  }
  throw new Error(`Timed out waiting for ${serverURL}`)
}

async function stopServer() {
  if (server.exitCode !== null) return
  server.kill('SIGTERM')
  await Promise.race([
    new Promise((resolve) => server.once('exit', resolve)),
    new Promise((resolve) => setTimeout(resolve, 2_000)),
  ])
  if (server.exitCode === null) server.kill('SIGKILL')
}

let exitCode = 1
try {
  await waitForServer()
  const env = { ...process.env, PLAYWRIGHT_EXTERNAL_SERVER: '1' }
  const windowsChrome = 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe'
  if (
    process.platform === 'win32'
    && !env.CI
    && !env.PLAYWRIGHT_USE_SYSTEM_CHROME
    && existsSync(windowsChrome)
  ) {
    env.PLAYWRIGHT_USE_SYSTEM_CHROME = '1'
  }

  exitCode = await new Promise((resolve, reject) => {
    const tests = spawn(
      process.execPath,
      [playwrightCli, 'test', ...process.argv.slice(2)],
      { stdio: 'inherit', env, windowsHide: true },
    )
    tests.once('error', reject)
    tests.once('exit', (code) => resolve(code ?? 1))
  })
} finally {
  await stopServer()
}

process.exitCode = exitCode
