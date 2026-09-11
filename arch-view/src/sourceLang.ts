const BY_EXT: Record<string, string> = {
  ts: 'typescript',
  mts: 'typescript',
  cts: 'typescript',
  tsx: 'tsx',
  js: 'javascript',
  mjs: 'javascript',
  cjs: 'javascript',
  jsx: 'jsx',
  go: 'go',
  cs: 'csharp',
  json: 'json',
  css: 'css',
  scss: 'scss',
  html: 'html',
  htm: 'html',
  md: 'markdown',
  py: 'python',
  sql: 'sql',
  yml: 'yaml',
  yaml: 'yaml',
  sh: 'bash',
  bash: 'bash',
  xml: 'xml',
  svg: 'xml',
}

export function languageFromPath(file: string): string {
  const base = file.replace(/\\/g, '/').split('/').pop() ?? file
  const dot = base.lastIndexOf('.')
  if (dot <= 0) return 'plaintext'
  const ext = base.slice(dot + 1).toLowerCase()
  return BY_EXT[ext] ?? 'plaintext'
}
