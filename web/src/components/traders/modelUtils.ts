export function getExchangeType(exchange: any): string {
  return exchange?.type || exchange?.exchange_type || ''
}
