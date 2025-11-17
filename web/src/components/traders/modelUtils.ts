// Helper utilities for model objects across different backend versions
export function getModelId(model: any): string {
  // Some APIs return { id } while older ones used { model_id }
  return (model && (model.model_id || model.id || model.ModelID)) || ''
}

export function getModelProvider(model: any): string {
  return model?.provider || model?.provider_name || ''
}

export function getExchangeId(exchange: any): string {
  // normalize exchange id across versions
  return (exchange && (exchange.exchange_id || exchange.id || exchange.ExchangeID)) || ''
}

export function getExchangeType(exchange: any): string {
  return exchange?.type || exchange?.exchange_type || ''
}
