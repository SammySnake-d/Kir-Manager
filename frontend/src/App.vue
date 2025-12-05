<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from './components/Icon.vue'

const { t, locale } = useI18n()

interface BackupItem {
  name: string
  backupTime: string
  hasToken: boolean
  hasMachineId: boolean
  machineId: string
  provider: string
  isCurrent: boolean
  isOriginalMachine: boolean // Machine ID 與原始機器相同
}

interface Result {
  success: boolean
  message: string
}

declare global {
  interface Window {
    go: {
      main: {
        App: {
          GetBackupList(): Promise<BackupItem[]>
          CreateBackup(name: string): Promise<Result>
          SwitchToBackup(name: string): Promise<Result>
          RestoreOriginal(): Promise<Result>
          DeleteBackup(name: string): Promise<Result>
          GetCurrentMachineID(): Promise<string>
          EnsureOriginalBackup(): Promise<Result>
          ResetToNewMachine(): Promise<Result>
          SoftResetToNewMachine(): Promise<Result>
          IsKiroRunning(): Promise<boolean>
          GetSoftResetStatus(): Promise<{
            isPatched: boolean
            hasCustomId: boolean
            customMachineId: string
            extensionPath: string
            isSupported: boolean
          }>
        }
      }
    }
  }
}

const backups = ref<BackupItem[]>([])
const currentMachineId = ref('')
const loading = ref(false)
const kiroRunning = ref(false)
const showCreateModal = ref(false)
const newBackupName = ref('')
const searchQuery = ref('')
const toast = ref<{ show: boolean; message: string; type: 'success' | 'error' }>({
  show: false,
  message: '',
  type: 'success'
})

// 一鍵新機模式相關
const resetMode = ref<'soft' | 'hard'>('soft')
const hasUsedReset = ref(false)
const showFirstTimeResetModal = ref(false)
const showSettingsPanel = ref(false)
const activeMenu = ref<'dashboard' | 'settings'>('dashboard')
const resetting = ref(false) // 一鍵新機進行中狀態

// 軟重置狀態
const softResetStatus = ref<{
  isPatched: boolean
  hasCustomId: boolean
  customMachineId: string
  extensionPath: string
  isSupported: boolean
}>({
  isPatched: false,
  hasCustomId: false,
  customMachineId: '',
  extensionPath: '',
  isSupported: false
})

const activeBackup = computed(() => {
  return backups.value.find(b => b.isCurrent) || null
})

const filteredBackups = computed(() => {
  if (!searchQuery.value.trim()) return backups.value
  const query = searchQuery.value.toLowerCase()
  return backups.value.filter(b => 
    b.name.toLowerCase().includes(query) ||
    b.machineId?.toLowerCase().includes(query) ||
    b.provider?.toLowerCase().includes(query)
  )
})

const switchLanguage = (lang: string) => {
  locale.value = lang
  localStorage.setItem('kiro-manager-lang', lang)
}

const showToast = (message: string, type: 'success' | 'error') => {
  toast.value = { show: true, message, type }
  setTimeout(() => {
    toast.value.show = false
  }, 3000)
}

const checkKiroStatus = async () => {
  try {
    kiroRunning.value = await window.go.main.App.IsKiroRunning()
  } catch (e) {
    console.error(e)
  }
}

const loadBackups = async () => {
  loading.value = true
  try {
    backups.value = await window.go.main.App.GetBackupList() || []
    currentMachineId.value = await window.go.main.App.GetCurrentMachineID()
    softResetStatus.value = await window.go.main.App.GetSoftResetStatus()
    await checkKiroStatus()
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

const createBackup = async () => {
  if (!newBackupName.value.trim()) return
  
  loading.value = true
  try {
    const result = await window.go.main.App.CreateBackup(newBackupName.value.trim())
    if (result.success) {
      showToast(t('message.success'), 'success')
      showCreateModal.value = false
      newBackupName.value = ''
      await loadBackups()
    } else {
      showToast(result.message, 'error')
    }
  } finally {
    loading.value = false
  }
}

const switchToBackup = async (name: string) => {
  if (!confirm(t('message.confirmSwitch', { name }))) return
  
  loading.value = true
  try {
    const result = await window.go.main.App.SwitchToBackup(name)
    if (result.success) {
      showToast(t('message.restartKiro'), 'success')
      await loadBackups()
    } else {
      showToast(result.message, 'error')
    }
  } finally {
    loading.value = false
  }
}

const restoreOriginal = async () => {
  if (!confirm(t('message.confirmRestore'))) return
  
  loading.value = true
  try {
    const result = await window.go.main.App.RestoreOriginal()
    if (result.success) {
      showToast(t('message.restartKiro'), 'success')
      await loadBackups()
    } else {
      showToast(result.message, 'error')
    }
  } finally {
    loading.value = false
  }
}

const resetToNew = async () => {
  // 首次使用時顯示提示 Modal
  if (!hasUsedReset.value && resetMode.value === 'soft') {
    showFirstTimeResetModal.value = true
    return
  }
  
  if (!confirm(t('message.confirmReset'))) return
  
  await executeReset()
}

const executeReset = async () => {
  resetting.value = true
  try {
    let result: Result
    if (resetMode.value === 'soft') {
      result = await window.go.main.App.SoftResetToNewMachine()
    } else {
      result = await window.go.main.App.ResetToNewMachine()
    }
    
    if (result.success) {
      showToast(result.message, 'success')
      // 標記已使用過一鍵新機
      hasUsedReset.value = true
      localStorage.setItem('kiro-manager-has-used-reset', 'true')
      await loadBackups()
    } else {
      showToast(result.message, 'error')
    }
  } finally {
    resetting.value = false
  }
}

const confirmFirstTimeReset = async () => {
  showFirstTimeResetModal.value = false
  if (!confirm(t('message.confirmReset'))) return
  await executeReset()
}

const setResetMode = (mode: 'soft' | 'hard') => {
  resetMode.value = mode
  localStorage.setItem('kiro-manager-reset-mode', mode)
}

const deleteBackup = async (name: string) => {
  if (!confirm(t('message.confirmDelete', { name }))) return
  
  loading.value = true
  try {
    const result = await window.go.main.App.DeleteBackup(name)
    if (result.success) {
      showToast(t('message.success'), 'success')
      await loadBackups()
    } else {
      showToast(result.message, 'error')
    }
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  // 語言已在 i18n/index.ts 中根據系統語言初始化
  // 這裡只需同步 locale 到當前組件（如果 localStorage 有值）
  const savedLang = localStorage.getItem('kiro-manager-lang')
  if (savedLang && ['zh-TW', 'zh-CN'].includes(savedLang)) {
    locale.value = savedLang
  }
  
  // 載入一鍵新機模式設定
  const savedResetMode = localStorage.getItem('kiro-manager-reset-mode')
  if (savedResetMode && ['soft', 'hard'].includes(savedResetMode)) {
    resetMode.value = savedResetMode as 'soft' | 'hard'
  }
  
  // 載入是否已使用過一鍵新機
  hasUsedReset.value = localStorage.getItem('kiro-manager-has-used-reset') === 'true'
  
  loadBackups()
  
  // 每 5 秒檢查一次 Kiro 運行狀態
  setInterval(checkKiroStatus, 5000)
})
</script>

<template>
  <div class="flex h-screen bg-app-bg font-sans text-sm text-zinc-300">
    
    <!-- 左側邊欄 -->
    <aside class="w-64 flex-shrink-0 border-r border-app-border flex flex-col bg-[#0c0c0e]">
      <div class="h-16 flex items-center px-6 border-b border-app-border">
        <!-- Kiro Logo SVG -->
        <svg width="28" height="28" viewBox="0 0 200 200" xmlns="http://www.w3.org/2000/svg" class="mr-3 flex-shrink-0">
          <defs>
            <linearGradient id="bgGradient" x1="0%" y1="0%" x2="0%" y2="100%">
              <stop offset="0%" style="stop-color:#2b3245;stop-opacity:1" />
              <stop offset="100%" style="stop-color:#1e222e;stop-opacity:1" />
            </linearGradient>
            <linearGradient id="kGradient" x1="0%" y1="0%" x2="100%" y2="100%">
              <stop offset="0%" style="stop-color:#61afef;stop-opacity:1" />
              <stop offset="100%" style="stop-color:#c678dd;stop-opacity:1" />
            </linearGradient>
            <filter id="dropShadow" x="-20%" y="-20%" width="140%" height="140%">
              <feGaussianBlur in="SourceAlpha" stdDeviation="3" />
              <feOffset dx="2" dy="4" result="offsetblur" />
              <feComponentTransfer>
                <feFuncA type="linear" slope="0.3" />
              </feComponentTransfer>
              <feMerge>
                <feMergeNode />
                <feMergeNode in="SourceGraphic" />
              </feMerge>
            </filter>
          </defs>
          <rect x="10" y="10" width="180" height="180" rx="40" ry="40" fill="url(#bgGradient)" stroke="#3e4451" stroke-width="2" />
          <circle cx="40" cy="40" r="6" fill="#ff5f56" />
          <circle cx="60" cy="40" r="6" fill="#ffbd2e" />
          <circle cx="80" cy="40" r="6" fill="#27c93f" />
          <g transform="translate(50, 70)" filter="url(#dropShadow)">
            <path d="M30 0 L0 40 L30 80" fill="none" stroke="url(#kGradient)" stroke-width="16" stroke-linecap="round" stroke-linejoin="round" />
            <line x1="35" y1="40" x2="75" y2="0" stroke="url(#kGradient)" stroke-width="16" stroke-linecap="round" />
            <line x1="35" y1="40" x2="65" y2="80" stroke="url(#kGradient)" stroke-width="16" stroke-linecap="round" />
            <rect x="85" y="70" width="20" height="10" fill="#98c379">
              <animate attributeName="opacity" values="1;0;1" dur="1s" repeatCount="indefinite" />
            </rect>
          </g>
        </svg>
        <span class="font-bold text-lg tracking-tight text-white">{{ t('app.name') }}</span>
      </div>
      
      <nav class="flex-1 p-4 space-y-1">
        <div 
          @click="activeMenu = 'dashboard'; showSettingsPanel = false"
          :class="[
            'px-3 py-2 rounded-lg flex items-center cursor-pointer transition-colors',
            activeMenu === 'dashboard' 
              ? 'text-zinc-100 bg-zinc-800/50 border border-zinc-700/50' 
              : 'text-zinc-500 hover:text-zinc-300 hover:bg-zinc-900'
          ]"
        >
          <Icon name="Layers" :class="['w-4 h-4 mr-3', activeMenu === 'dashboard' ? 'text-app-accent' : '']" />
          {{ t('menu.dashboard') }}
        </div>
        <div 
          @click="activeMenu = 'settings'; showSettingsPanel = true"
          :class="[
            'px-3 py-2 rounded-lg flex items-center cursor-pointer transition-colors',
            activeMenu === 'settings' 
              ? 'text-zinc-100 bg-zinc-800/50 border border-zinc-700/50' 
              : 'text-zinc-500 hover:text-zinc-300 hover:bg-zinc-900'
          ]"
        >
          <Icon name="Cpu" :class="['w-4 h-4 mr-3', activeMenu === 'settings' ? 'text-app-accent' : '']" />
          {{ t('menu.settings') }}
        </div>
      </nav>

    </aside>

    <!-- 右側主內容 -->
    <main class="flex-1 flex flex-col min-w-0 overflow-hidden bg-app-bg relative">
      <!-- 頂部標題列 -->
      <header class="h-16 border-b border-app-border flex items-center justify-between px-8 glass sticky top-0 z-10">
        <div>
          <h2 class="text-white font-semibold text-lg">{{ showSettingsPanel ? t('settings.title') : t('menu.dashboard') }}</h2>
          <p class="text-zinc-500 text-xs">{{ t('app.systemReady') }} • {{ t('app.version') }}</p>
        </div>
        <div class="flex items-center gap-2">
          <div :class="['w-2 h-2 rounded-full', loading ? 'bg-yellow-500 animate-pulse' : kiroRunning ? 'bg-green-500' : 'bg-zinc-500']"></div>
          <span class="text-xs text-zinc-400 font-mono">{{ loading ? t('app.processing') : kiroRunning ? t('app.kiroRunning') : t('app.kiroStopped') }}</span>
        </div>
      </header>

      <!-- 內容滾動區 -->
      <div class="flex-1 overflow-y-auto p-8 space-y-8">
        
        <!-- 設定面板 -->
        <div v-if="showSettingsPanel" class="max-w-2xl">
            <h3 class="text-white font-semibold text-xl mb-6">{{ t('settings.title') }}</h3>
            
            <!-- 語言設定 -->
            <div class="bg-zinc-900 border border-app-border rounded-xl p-6 mb-6">
              <h4 class="text-zinc-300 font-medium mb-4 flex items-center">
                <Icon name="Layers" class="w-5 h-5 mr-2 text-zinc-400" />
                {{ t('settings.language') }}
              </h4>
              
              <div class="flex gap-3">
                <button 
                  v-for="lang in ['zh-TW', 'zh-CN']" 
                  :key="lang"
                  @click="switchLanguage(lang)"
                  :class="[
                    'flex-1 py-3 px-4 rounded-lg border transition-all text-sm',
                    locale === lang 
                      ? 'bg-zinc-800 border-zinc-600 text-zinc-200' 
                      : 'border-zinc-700 hover:border-zinc-600 text-zinc-400 hover:text-zinc-300'
                  ]"
                >
                  {{ lang === 'zh-TW' ? t('language.zhTW') : t('language.zhCN') }}
                </button>
              </div>
            </div>
            
            <!-- 一鍵新機模式設定 -->
            <div class="bg-zinc-900 border border-app-border rounded-xl p-6">
              <h4 class="text-zinc-300 font-medium mb-4 flex items-center">
                <Icon name="Sparkles" class="w-5 h-5 mr-2 text-zinc-400" />
                {{ t('settings.resetMode') }}
              </h4>
              
              <div class="space-y-3">
                <!-- 軟一鍵新機選項 -->
                <label 
                  @click="setResetMode('soft')"
                  :class="[
                    'flex items-start p-4 rounded-lg border cursor-pointer transition-all',
                    resetMode === 'soft' 
                      ? 'bg-zinc-800 border-zinc-600' 
                      : 'border-zinc-700 hover:border-zinc-600'
                  ]"
                >
                  <div :class="[
                    'w-5 h-5 rounded-full border-2 flex items-center justify-center mr-4 mt-0.5 flex-shrink-0',
                    resetMode === 'soft' ? 'border-zinc-400' : 'border-zinc-500'
                  ]">
                    <div v-if="resetMode === 'soft'" class="w-2.5 h-2.5 rounded-full bg-zinc-400"></div>
                  </div>
                  <div class="flex-1">
                    <div class="flex items-center gap-2 mb-1">
                      <span :class="['font-medium', resetMode === 'soft' ? 'text-zinc-200' : 'text-zinc-300']">
                        {{ t('settings.softReset') }}
                      </span>
                      <span class="px-1.5 py-0.5 rounded text-[10px] bg-app-success/20 text-app-success border border-app-success/30">
                        {{ t('settings.recommended') }}
                      </span>
                    </div>
                    <p class="text-zinc-500 text-sm">{{ t('settings.softResetDesc') }}</p>
                  </div>
                </label>
                
                <!-- 硬一鍵新機選項 -->
                <label 
                  @click="setResetMode('hard')"
                  :class="[
                    'flex items-start p-4 rounded-lg border cursor-pointer transition-all',
                    resetMode === 'hard' 
                      ? 'bg-zinc-800 border-zinc-600' 
                      : 'border-zinc-700 hover:border-zinc-600'
                  ]"
                >
                  <div :class="[
                    'w-5 h-5 rounded-full border-2 flex items-center justify-center mr-4 mt-0.5 flex-shrink-0',
                    resetMode === 'hard' ? 'border-zinc-400' : 'border-zinc-500'
                  ]">
                    <div v-if="resetMode === 'hard'" class="w-2.5 h-2.5 rounded-full bg-zinc-400"></div>
                  </div>
                  <div class="flex-1">
                    <div class="flex items-center gap-2 mb-1">
                      <span :class="['font-medium', resetMode === 'hard' ? 'text-zinc-200' : 'text-zinc-300']">
                        {{ t('settings.hardReset') }}
                      </span>
                      <span class="px-1.5 py-0.5 rounded text-[10px] bg-app-warning/20 text-app-warning border border-app-warning/30">
                        {{ t('settings.windowsOnly') }}
                      </span>
                    </div>
                    <p class="text-zinc-500 text-sm">{{ t('settings.hardResetDesc') }}</p>
                  </div>
                </label>
              </div>
            </div>
        </div>
        
        <!-- Dashboard 內容 -->
        <div v-else class="space-y-8">
        
        <!-- 當前狀態 + 操作按鈕 -->
        <div class="grid grid-cols-1 lg:grid-cols-5 gap-6">
          
          <!-- 當前狀態卡片 -->
          <div class="lg:col-span-3 bg-gradient-to-br from-zinc-900 to-zinc-900/50 border border-app-border rounded-xl p-6 relative overflow-hidden group">
            <div class="absolute top-0 right-0 p-4 opacity-10 group-hover:opacity-20 transition-opacity">
              <Icon name="Cpu" class="w-32 h-32 text-white" />
            </div>
            
            <div class="relative z-10">
              <div class="flex items-center gap-2 mb-4">
                <span class="px-2 py-0.5 rounded text-[10px] font-bold bg-app-warning text-black uppercase tracking-wider">
                  {{ t('status.current') }}
                </span>
                <span v-if="activeBackup?.backupTime" class="text-zinc-500 text-xs font-mono">
                  {{ activeBackup.backupTime }}
                </span>
              </div>
              
              <h3 class="text-3xl font-bold text-white mb-1 glow-text">{{ activeBackup?.name || t('status.originalMachine') }}</h3>
              <div class="flex items-center gap-2 text-app-accent font-mono text-sm mb-6">
                <Icon name="Check" class="w-4 h-4" />
                {{ currentMachineId || '-' }}
              </div>

              <div class="flex gap-3">
                <button 
                  @click="showCreateModal = true"
                  class="flex items-center px-4 py-2 bg-zinc-800 hover:bg-zinc-700 border border-zinc-600 text-zinc-200 rounded-lg text-sm transition-all active:scale-95"
                >
                  <Icon name="Save" class="w-4 h-4 mr-2" />
                  {{ t('backup.create') }}
                </button>
                <button 
                  @click="restoreOriginal"
                  class="flex items-center px-4 py-2 bg-zinc-800/50 hover:bg-red-900/30 border border-zinc-700/50 hover:border-red-800/50 text-zinc-400 hover:text-red-400 rounded-lg text-sm transition-all"
                >
                  <Icon name="Rotate" class="w-4 h-4 mr-2" />
                  {{ t('restore.original') }}
                </button>
              </div>
            </div>
          </div>

          <!-- PATCH 狀態 + 一鍵新機合併卡片 -->
          <div class="lg:col-span-2 bg-zinc-900 border border-app-border rounded-xl p-4 flex flex-col">
            <!-- 上方：PATCH 狀態 -->
            <div class="flex items-center gap-2 mb-3">
              <Icon name="Cpu" class="w-4 h-4 text-zinc-400" />
              <span class="text-zinc-400 text-xs font-semibold uppercase tracking-wider">{{ t('status.patchStatus') }}</span>
            </div>
            
            <div class="space-y-2 mb-4">
              <!-- Patch 狀態 -->
              <div class="flex items-center justify-between">
                <span class="text-zinc-500 text-sm">Extension Patch</span>
                <span :class="[
                  'px-2 py-0.5 rounded text-xs font-medium',
                  softResetStatus.isPatched 
                    ? 'bg-app-success/20 text-app-success border border-app-success/30' 
                    : 'bg-zinc-700/50 text-zinc-400 border border-zinc-600/30'
                ]">
                  {{ softResetStatus.isPatched ? t('status.patched') : t('status.notPatched') }}
                </span>
              </div>
              
              <!-- 自訂 ID 狀態 -->
              <div class="flex items-center justify-between">
                <span class="text-zinc-500 text-sm">Machine ID</span>
                <span :class="[
                  'px-2 py-0.5 rounded text-xs font-medium',
                  softResetStatus.hasCustomId 
                    ? 'bg-app-accent/20 text-app-accent border border-app-accent/30' 
                    : 'bg-zinc-700/50 text-zinc-400 border border-zinc-600/30'
                ]">
                  {{ softResetStatus.hasCustomId ? t('status.hasCustomId') : t('status.noCustomId') }}
                </span>
              </div>
              
              <!-- 總體狀態指示 -->
              <div class="flex items-center gap-2 pt-1">
                <div :class="[
                  'w-2 h-2 rounded-full',
                  softResetStatus.isPatched && softResetStatus.hasCustomId 
                    ? 'bg-app-success shadow-[0_0_6px_rgba(34,197,94,0.6)]' 
                    : 'bg-zinc-500'
                ]"></div>
                <span :class="[
                  'text-xs font-medium',
                  softResetStatus.isPatched && softResetStatus.hasCustomId 
                    ? 'text-app-success' 
                    : 'text-zinc-500'
                ]">
                  {{ softResetStatus.isPatched && softResetStatus.hasCustomId ? t('status.softResetActive') : t('status.softResetInactive') }}
                </span>
              </div>
            </div>
            
            <!-- 下方：一鍵新機按鈕 -->
            <button 
              @click="resetToNew"
              :disabled="resetting"
              :class="[
                'mt-auto relative group flex items-center justify-center gap-3 px-4 py-3 border rounded-lg transition-all',
                resetting 
                  ? 'bg-app-accent border-app-accent cursor-wait' 
                  : 'bg-zinc-800 hover:bg-app-accent border-zinc-700 hover:border-app-accent active:scale-95'
              ]"
            >
              <!-- 一鍵新機 SVG Icon -->
              <svg width="32" height="32" viewBox="0 0 100 100" fill="none" xmlns="http://www.w3.org/2000/svg" class="flex-shrink-0">
                <!-- 手機主體 (靜態) -->
                <rect x="25" y="15" width="50" height="80" rx="6" stroke="currentColor" stroke-width="4" fill="none"/>
                <line x1="42" y1="22" x2="58" y2="22" stroke="currentColor" stroke-width="3" stroke-linecap="round"/>
                <circle cx="50" cy="85" r="3" fill="currentColor"/>
                
                <!-- 進行中：只顯示持續旋轉的箭頭 -->
                <g v-if="resetting">
                  <path d="M50 40 A 15 15 0 1 1 38 63" stroke="currentColor" stroke-width="3" stroke-linecap="round" fill="none" />
                  <path d="M38 63 L34 58 M38 63 L43 59" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"/>
                  <animateTransform attributeName="transform" type="rotate" from="0 50 55" to="360 50 55" dur="0.6s" repeatCount="indefinite" />
                </g>
                
                <!-- 靜態：不旋轉的箭頭 -->
                <g v-else>
                  <path d="M50 40 A 15 15 0 1 1 38 63" stroke="currentColor" stroke-width="3" stroke-linecap="round" fill="none" />
                  <path d="M38 63 L34 58 M38 63 L43 59" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"/>
                </g>
              </svg>
              <div class="text-left">
                <span :class="['text-sm font-bold block', resetting ? 'text-white' : 'text-zinc-200 group-hover:text-white']">
                  {{ resetting ? t('app.processing') : t('restore.reset') }}
                </span>
                <span :class="['text-[10px]', resetting ? 'text-zinc-200' : 'text-zinc-500 group-hover:text-zinc-300']">
                  {{ resetting ? t('message.restartKiro') : t('restore.resetDesc') }}
                </span>
              </div>
            </button>
          </div>
        </div>

        <!-- 表格區域 -->
        <div>
          <div class="flex items-center justify-between mb-4">
            <h3 class="text-zinc-400 text-sm font-semibold flex items-center">
              <Icon name="Layers" class="w-4 h-4 mr-2" />
              {{ t('backup.list') }}
            </h3>
            <div class="relative">
              <Icon name="Search" class="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-zinc-500" />
              <input 
                v-model="searchQuery"
                :placeholder="t('backup.search')"
                class="pl-9 pr-4 py-1.5 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-200 text-sm focus:outline-none focus:border-app-accent transition-colors w-48"
              />
            </div>
          </div>
          
          <div class="bg-app-surface border border-app-border rounded-xl overflow-hidden shadow-xl">
            <table class="w-full text-left border-collapse">
              <thead>
                <tr class="border-b border-zinc-800 bg-zinc-900/50 text-zinc-500 text-xs uppercase tracking-wider">
                  <th class="px-6 py-4 font-medium">{{ t('backup.name') }}</th>
                  <th class="px-6 py-4 font-medium">{{ t('backup.provider') }}</th>
                  <th class="px-6 py-4 font-medium">{{ t('backup.machineId') }}</th>
                  <th class="px-6 py-4 font-medium text-right">{{ t('backup.actions') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-zinc-800/50">
                <tr v-if="filteredBackups.length === 0">
                  <td colspan="4" class="px-6 py-12 text-center text-zinc-500">{{ t('backup.noBackups') }}</td>
                </tr>
                <tr 
                  v-for="backup in filteredBackups" 
                  :key="backup.name"
                  :class="['group transition-colors', backup.isCurrent ? 'bg-app-accent/5' : 'hover:bg-zinc-800/30']"
                >
                  <td class="px-6 py-4">
                    <div class="flex items-center">
                      <div v-if="backup.isCurrent" class="w-1.5 h-1.5 rounded-full bg-app-warning mr-3 shadow-[0_0_8px_rgba(245,158,11,0.8)]"></div>
                      <span :class="['font-medium', backup.isCurrent ? 'text-white' : 'text-zinc-400 group-hover:text-zinc-300']">
                        {{ backup.name }}
                      </span>
                      <span v-if="backup.isOriginalMachine" class="ml-2 px-1.5 py-0.5 rounded text-[10px] bg-app-accent/20 text-app-accent border border-app-accent/30">
                        {{ t('backup.original') }}
                      </span>
                    </div>
                  </td>
                  <td class="px-6 py-4">
                    <span class="px-2 py-1 rounded text-[10px] bg-zinc-800 text-zinc-400 border border-zinc-700">
                      {{ backup.provider || '-' }}
                    </span>
                  </td>
                  <td class="px-6 py-4 font-mono text-xs text-zinc-500">
                    {{ backup.machineId || '-' }}
                  </td>
                  <td class="px-6 py-4 text-right">
                    <div v-if="backup.isCurrent" class="text-app-warning text-xs font-bold flex items-center justify-end gap-1">
                      <div class="w-1 h-1 bg-app-warning rounded-full animate-ping"></div>
                      {{ t('status.active') }}
                    </div>
                    <div v-else class="flex items-center justify-end gap-2">
                      <button 
                        @click="switchToBackup(backup.name)"
                        class="text-xs bg-transparent border border-zinc-700 hover:border-zinc-500 text-zinc-400 hover:text-white px-3 py-1.5 rounded transition-all"
                      >
                        {{ t('backup.switchTo') }}
                      </button>
                      <button 
                        @click="deleteBackup(backup.name)"
                        class="text-xs bg-transparent border border-zinc-700 hover:border-red-700 text-zinc-400 hover:text-red-400 px-2 py-1.5 rounded transition-all"
                      >
                        <Icon name="Trash" class="w-3 h-3" />
                      </button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
        
        </div>
      </div>

      <!-- Loading 遮罩 -->
      <div v-if="loading" class="absolute inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center">
        <div class="flex flex-col items-center">
          <div class="w-10 h-10 border-4 border-app-accent border-t-transparent rounded-full animate-spin mb-4"></div>
          <span class="text-white text-sm font-medium tracking-widest">PROCESSING</span>
        </div>
      </div>
    </main>

    <!-- Create Modal -->
    <div v-if="showCreateModal" class="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center z-50" @click.self="showCreateModal = false">
      <div class="bg-app-surface border border-app-border rounded-xl p-6 min-w-[400px] shadow-2xl">
        <h3 class="text-white font-semibold text-lg mb-4">{{ t('backup.createTitle') }}</h3>
        <div class="mb-4">
          <label class="block text-zinc-400 text-sm mb-2">{{ t('backup.nameLabel') }}</label>
          <input 
            v-model="newBackupName" 
            :placeholder="t('backup.namePlaceholder')"
            @keyup.enter="createBackup"
            class="w-full px-4 py-2 bg-zinc-900 border border-zinc-700 rounded-lg text-zinc-200 text-sm focus:outline-none focus:border-app-accent transition-colors"
          />
        </div>
        <div class="flex justify-end gap-3">
          <button 
            @click="showCreateModal = false"
            class="px-4 py-2 bg-zinc-800 hover:bg-zinc-700 text-zinc-300 rounded-lg text-sm transition-colors"
          >
            {{ t('backup.cancel') }}
          </button>
          <button 
            @click="createBackup"
            class="px-4 py-2 bg-app-accent hover:bg-app-accent/80 text-white rounded-lg text-sm transition-colors"
          >
            {{ t('backup.confirm') }}
          </button>
        </div>
      </div>
    </div>

    <!-- First Time Reset Modal -->
    <div v-if="showFirstTimeResetModal" class="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center z-50" @click.self="showFirstTimeResetModal = false">
      <div class="bg-app-surface border border-app-border rounded-xl p-6 max-w-md shadow-2xl">
        <div class="flex items-center gap-3 mb-4">
          <div class="w-10 h-10 rounded-full bg-app-accent/20 flex items-center justify-center">
            <Icon name="Sparkles" class="w-5 h-5 text-app-accent" />
          </div>
          <h3 class="text-white font-semibold text-lg">{{ t('message.firstTimeResetTitle') }}</h3>
        </div>
        
        <div class="space-y-4 mb-6">
          <p class="text-zinc-300 text-sm leading-relaxed">
            {{ t('message.firstTimeResetInfo') }}
          </p>
          <div class="bg-zinc-800/50 border border-zinc-700 rounded-lg p-3">
            <p class="text-zinc-400 text-xs leading-relaxed">
              💡 {{ t('message.firstTimeResetTip') }}
            </p>
          </div>
        </div>
        
        <div class="flex justify-end gap-3">
          <button 
            @click="showFirstTimeResetModal = false"
            class="px-4 py-2 bg-zinc-800 hover:bg-zinc-700 text-zinc-300 rounded-lg text-sm transition-colors"
          >
            {{ t('backup.cancel') }}
          </button>
          <button 
            @click="confirmFirstTimeReset"
            class="px-4 py-2 bg-app-accent hover:bg-app-accent/80 text-white rounded-lg text-sm transition-colors"
          >
            {{ t('message.continueReset') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Toast -->
    <Transition name="slide">
      <div 
        v-if="toast.show" 
        :class="[
          'fixed bottom-5 right-5 px-5 py-3 rounded-lg text-white text-sm z-50 shadow-lg',
          toast.type === 'success' ? 'bg-app-success' : 'bg-app-danger'
        ]"
      >
        {{ toast.message }}
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.slide-enter-active,
.slide-leave-active {
  transition: all 0.3s ease;
}
.slide-enter-from,
.slide-leave-to {
  transform: translateX(100%);
  opacity: 0;
}
</style>
