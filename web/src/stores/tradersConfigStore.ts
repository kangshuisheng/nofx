import { create } from 'zustand'
import type { AIModel, Exchange } from '../types'
import { api } from '../lib/api'

interface SignalSource {
  coinPoolUrl: string
  oiTopUrl: string
}

interface TradersConfigState {
  // 数据
  allModels: AIModel[]
  allExchanges: Exchange[]
  supportedModels: AIModel[]
  supportedExchanges: Exchange[]
  userSignalSource: SignalSource

  // 计算属性
  configuredModels: AIModel[]
  configuredExchanges: Exchange[]

  // Actions
  setAllModels: (models: AIModel[]) => void
  setAllExchanges: (exchanges: Exchange[]) => void
  setSupportedModels: (models: AIModel[]) => void
  setSupportedExchanges: (exchanges: Exchange[]) => void
  setUserSignalSource: (source: SignalSource) => void

  // 异步加载
  loadConfigs: (user: any, token: string | null) => Promise<void>

  // 重置
  reset: () => void
}

const initialState = {
  allModels: [],
  allExchanges: [],
  supportedModels: [],
  supportedExchanges: [],
  userSignalSource: { coinPoolUrl: '', oiTopUrl: '' },
  configuredModels: [],
  configuredExchanges: [],
}

export const useTradersConfigStore = create<TradersConfigState>((set, get) => ({
  ...initialState,

  setAllModels: (models) => {
    set({ allModels: models })
    // 更新 configuredModels - 顯示所有已啟用或有自定義配置的模型
    // 注意：後端不返回 apiKey，只能通過 enabled 和 customApiUrl 判斷
    const configuredModels = models.filter((m) => {
      return m.enabled || (m.customApiUrl && m.customApiUrl.trim() !== '')
    })
    set({ configuredModels })
  },

  setAllExchanges: (exchanges) => {
    set({ allExchanges: exchanges })
    // 更新 configuredExchanges - 顯示所有已啟用或有配置資料的交易所
    // 注意：後端不返回 apiKey/secretKey/asterPrivateKey 等敏感字段
    const configuredExchanges = exchanges.filter((e) => {
      // 主要依據 enabled 狀態判斷
      if (e.enabled) return true

      // 額外檢查：如果有非敏感配置字段，也認為是已配置
      if (e.id === 'aster') {
        return (
          (e.asterUser && e.asterUser.trim() !== '') ||
          (e.asterSigner && e.asterSigner.trim() !== '')
        )
      }
      if (e.id === 'hyperliquid') {
        return e.hyperliquidWalletAddr && e.hyperliquidWalletAddr.trim() !== ''
      }

      // 其他交易所只看 enabled
      return false
    })
    set({ configuredExchanges })
  },

  setSupportedModels: (models) => set({ supportedModels: models }),
  setSupportedExchanges: (exchanges) => set({ supportedExchanges: exchanges }),
  setUserSignalSource: (source) => set({ userSignalSource: source }),

  loadConfigs: async (user, token) => {
    if (!user || !token) {
      // 未登录时只加载公开的支持模型和交易所
      try {
        const [supportedModels, supportedExchanges] = await Promise.all([
          api.getSupportedModels(),
          api.getSupportedExchanges(),
        ])
        get().setSupportedModels(supportedModels)
        get().setSupportedExchanges(supportedExchanges)
      } catch (err) {
        console.error('Failed to load supported configs:', err)
      }
      return
    }

    try {
      const [
        modelConfigs,
        exchangeConfigs,
        supportedModels,
        supportedExchanges,
      ] = await Promise.all([
        api.getModelConfigs(),
        api.getExchangeConfigs(),
        api.getSupportedModels(),
        api.getSupportedExchanges(),
      ])

      get().setAllModels(modelConfigs)
      get().setAllExchanges(exchangeConfigs)
      get().setSupportedModels(supportedModels)
      get().setSupportedExchanges(supportedExchanges)

      // 加载用户信号源配置
      try {
        const signalSource = await api.getUserSignalSource()
        get().setUserSignalSource({
          coinPoolUrl: signalSource.coin_pool_url || '',
          oiTopUrl: signalSource.oi_top_url || '',
        })
      } catch (error) {
        console.log('📡 用户信号源配置暂未设置')
      }
    } catch (error) {
      console.error('Failed to load configs:', error)
    }
  },

  reset: () => set(initialState),
}))
