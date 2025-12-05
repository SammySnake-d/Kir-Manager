import { createI18n } from 'vue-i18n'
import zhTW from './zh-TW'
import zhCN from './zh-CN'

/**
 * 偵測系統語言並返回對應的 locale
 * 優先順序：localStorage > 系統語言 > 預設 zh-TW
 */
function getDefaultLocale(): string {
  // 1. 優先使用用戶已保存的語言偏好
  const savedLang = localStorage.getItem('kiro-manager-lang')
  if (savedLang && ['zh-TW', 'zh-CN'].includes(savedLang)) {
    return savedLang
  }

  // 2. 偵測系統/瀏覽器語言
  const systemLang = navigator.language || (navigator.languages && navigator.languages[0]) || ''
  const langLower = systemLang.toLowerCase()

  // 簡體中文判斷：zh-cn, zh-hans, zh-sg (新加坡簡體)
  if (langLower === 'zh-cn' || 
      langLower === 'zh-hans' || 
      langLower.startsWith('zh-hans') ||
      langLower === 'zh-sg') {
    return 'zh-CN'
  }

  // 3. 預設繁體中文
  return 'zh-TW'
}

const i18n = createI18n({
  legacy: false,
  locale: getDefaultLocale(),
  fallbackLocale: 'zh-TW',
  messages: {
    'zh-TW': zhTW,
    'zh-CN': zhCN,
  },
})

export default i18n
