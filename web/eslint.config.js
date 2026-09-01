import js from '@eslint/js'
import autoImportGlobals from './.eslintrc-auto-import.json' with { type: 'json' }
import eslintConfigPrettier from 'eslint-config-prettier'
import globals from 'globals'
import pluginVue from 'eslint-plugin-vue'

export default [
  {
    ignores: ['dist/', 'node_modules/'],
  },
  js.configs.recommended,
  ...pluginVue.configs['flat/recommended'],
  {
    files: ['**/*.{js,vue}'],
    languageOptions: {
      globals: {
        ...globals.browser,
        ...autoImportGlobals.globals,
      },
    },
    rules: {
      'vue/multi-word-component-names': 'off',
    },
  },
  eslintConfigPrettier,
]
