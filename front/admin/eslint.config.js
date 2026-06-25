import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import tseslint from 'typescript-eslint'
import { defineConfig, globalIgnores } from 'eslint/config'

export default defineConfig([
  globalIgnores(['dist']),
  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommended,
      reactHooks.configs.flat.recommended,
      reactRefresh.configs.vite,
    ],
    languageOptions: {
      globals: globals.browser,
    },
    rules: {
      // The React-Compiler-oriented rules added in eslint-plugin-react-hooks v6
      // error on idiomatic patterns this app relies on:
      //  - set-state-in-effect: fetch-on-mount + theme/active-tab sync effects.
      //  - static-components: the `const Icon = resolveIcon(...); <Icon/>` dynamic
      //    icon pattern used throughout the nav/menus (a false positive here).
      // They are disabled until/unless we adopt the React Compiler. exhaustive-deps
      // stays on as a warning.
      'react-hooks/set-state-in-effect': 'off',
      'react-hooks/static-components': 'off',
    },
  },
])
