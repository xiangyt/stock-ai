/** 外部行情站点跳转链接（东方财富 / 同花顺 / 腾讯自选股） */

/** 按股票代码首字符推断交易所前缀 */
export function getExchangePrefix(code: string): string {
  const c = code.charAt(0)
  if (c === '6') return 'sh'
  if (c === '0' || c === '3') return 'sz'
  if (c === '8' || c === '9') return 'bj'
  return 'sz'
}

/** 东方财富个股页面 */
export function getEastMoneyUrl(code: string): string {
  return `https://quote.eastmoney.com/concept/${getExchangePrefix(code)}${code}.html#chart-k-cyq`
}

/** 同花顺 iwencai 个股页面 */
export function getTHSUrl(code: string): string {
  return `https://www.iwencai.com/screener/result?w=${code}&querytype=stock&sign=1781436668603`
}

/** 腾讯自选股个股页面 */
export function getTencentUrl(code: string): string {
  return `https://gu.qq.com/${getExchangePrefix(code)}${code}/gp`
}
