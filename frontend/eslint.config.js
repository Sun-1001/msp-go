import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import tseslint from 'typescript-eslint'
import { defineConfig, globalIgnores } from 'eslint/config'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const srcRoot = fileURLToPath(new URL('./src', import.meta.url))
const modulesRoot = path.join(srcRoot, 'modules')
const publicModuleEntries = {
  admin: ['api', 'state', 'layout', 'dashboard', 'users', 'settings'],
  'ai-config': ['state'],
  announcement: ['api', 'notifications'],
  auth: ['api', 'state', 'routing', 'navigation', 'hooks', 'captcha'],
  'daily-question': ['api', 'teacher'],
  email: ['api'],
  exercise: ['api', 'state', 'provider', 'tutor', 'math'],
  forum: ['api'],
  curriculum: ['api'],
  'message-center': ['api', 'notifications', 'shared'],
  mistake: ['api', 'state'],
  resource: ['api', 'state'],
  session: ['api', 'state'],
  student: ['api', 'state'],
}

function moduleFile(filename) {
  const relative = path.relative(modulesRoot, filename)
  if (path.isAbsolute(relative) || relative === '..' || relative.startsWith(`..${path.sep}`)) return null
  const [name, ...parts] = relative.split(path.sep)
  return { name, entry: parts.join('/').replace(/\.[cm]?[jt]sx?$/, '') || 'index' }
}

const modulePublicApi = {
  meta: {
    type: 'problem',
    schema: [],
    messages: {
      privateImport: '模块 {{name}} 的实现文件不是公共接口，请使用 @/modules/{{name}} 或已登记的公共子入口。',
      selfImport: '模块内部不要反向导入自身 index.ts，请直接引用内部文件以避免循环依赖。',
    },
  },
  create(context) {
    const owner = moduleFile(context.filename)
    function check(source) {
      const specifier = source?.type === 'TemplateLiteral' && source.expressions.length === 0
        ? source.quasis[0].value.cooked
        : source?.value
      if (typeof specifier !== 'string') return
      const request = specifier.split(/[?#]/)[0]
      const filename = request.startsWith('@/')
        ? path.resolve(srcRoot, request.slice(2))
        : request.startsWith('.')
          ? path.resolve(path.dirname(context.filename), request)
          : null
      if (!filename) return
      const target = moduleFile(filename)
      if (!target) return
      if (target.name === owner?.name) {
        if (target.entry === 'index') context.report({ node: source, messageId: 'selfImport' })
        return
      }
      const entries = Object.hasOwn(publicModuleEntries, target.name) ? publicModuleEntries[target.name] : []
      if (target.entry !== 'index' && !entries.includes(target.entry)) {
        context.report({ node: source, messageId: 'privateImport', data: { name: target.name } })
      }
    }
    return {
      ImportDeclaration: (node) => check(node.source),
      ExportNamedDeclaration: (node) => check(node.source),
      ExportAllDeclaration: (node) => check(node.source),
      ImportExpression: (node) => check(node.source),
      TSImportType: (node) => check(node.argument?.literal ?? node.argument ?? node.source),
      TSExternalModuleReference: (node) => check(node.expression),
      CallExpression: (node) => {
        if (node.callee.type === 'Identifier' && node.callee.name === 'require') check(node.arguments[0])
      },
    }
  },
}

export default defineConfig([
  globalIgnores(['dist', '**/._*']),
  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommended,
      reactHooks.configs.flat.recommended,
      reactRefresh.configs.vite,
    ],
    languageOptions: {
      ecmaVersion: 2020,
      globals: globals.browser,
    },
  },
  {
    files: ['src/**/*.{ts,tsx}'],
    plugins: { architecture: { rules: { 'module-public-api': modulePublicApi } } },
    rules: { 'architecture/module-public-api': 'error' },
  },
])
