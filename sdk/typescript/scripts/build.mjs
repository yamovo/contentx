import { mkdirSync, rmSync, writeFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawnSync } from 'node:child_process'

const sdkRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const distDir = resolve(sdkRoot, 'dist')

if (dirname(distDir) !== sdkRoot) {
  throw new Error(`Refusing to clean unexpected output directory: ${distDir}`)
}

rmSync(distDir, { recursive: true, force: true })

const tsc = resolve(sdkRoot, 'node_modules', 'typescript', 'bin', 'tsc')
for (const config of ['tsconfig.esm.json', 'tsconfig.cjs.json']) {
  const result = spawnSync(process.execPath, [tsc, '--project', config], {
    cwd: sdkRoot,
    stdio: 'inherit',
  })
  if (result.status !== 0) {
    process.exit(result.status ?? 1)
  }
}

const cjsDir = resolve(distDir, 'cjs')
mkdirSync(cjsDir, { recursive: true })
writeFileSync(resolve(cjsDir, 'package.json'), '{"type":"commonjs"}\n')
