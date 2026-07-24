/* eslint-env node */
module.exports = {
  root: true,
  env: {
    browser: true,
    es2021: true,
    node: true,
  },
  extends: [
    'eslint:recommended',
    'plugin:@typescript-eslint/recommended',
    'plugin:vue/vue3-recommended',
  ],
  parser: 'vue-eslint-parser',
  parserOptions: {
    parser: '@typescript-eslint/parser',
    ecmaVersion: 'latest',
    sourceType: 'module',
  },
  rules: {
    // Allow unused args prefixed with _ (common convention in Vue handlers).
    '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
    // Permit `any` in existing code during incremental cleanup; A-23 will tighten.
    '@typescript-eslint/no-explicit-any': 'off',
    // Allow empty catch blocks (intentional fire-and-forget pattern).
    'no-empty': ['error', { allowEmptyCatch: true }],
    // Vue: allow v-html (sanitized via DOMPurify at call sites) and multi-word
    // component names are noisy for single-page views.
    'vue/no-v-html': 'off',
    'vue/multi-word-component-names': 'off',
    // Form state is held in a reactive object owned by the parent editor and
    // passed to child sidebar/panel components; mutating its fields via v-model
    // is the intended two-way binding pattern.
    'vue/no-mutating-props': 'off',
    // Prefer consistent but not dogmatic formatting.
    'vue/attributes-order': 'warn',
    'vue/html-self-closing': 'warn',
    // Keep no-undef off for TS files (handled by the TS parser).
    'no-undef': 'off',
  },
  ignorePatterns: ['dist/', 'node_modules/', '*.d.ts', 'coverage/'],
}
