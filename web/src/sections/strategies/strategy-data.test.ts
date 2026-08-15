// 策略币源序列化纯函数测试（strToCoins 兼容全角逗号，2026-08 实锤）。
import { describe, it, expect } from 'vitest';
import { coinsToStr, strToCoins } from './strategy-data';

describe('strToCoins', () => {
  it('半角逗号分隔并大写', () => {
    expect(strToCoins('btc-usdt, eth-usdt')).toEqual(['BTC-USDT', 'ETH-USDT']);
  });
  it('中文全角逗号同样切分（2026-08 实锤：全角逗号曾导致单元素畸形符号）', () => {
    expect(strToCoins('UNI-USDT，BTC-USDT')).toEqual(['UNI-USDT', 'BTC-USDT']);
  });
  it('半角+全角混用、空格、空项都处理', () => {
    expect(strToCoins(' btc-usdt，eth-usdt , sol-usdt, ')).toEqual(['BTC-USDT', 'ETH-USDT', 'SOL-USDT']);
  });
  it('空串返回空数组', () => {
    expect(strToCoins('')).toEqual([]);
  });
});

describe('coinsToStr', () => {
  it('数组拼接为逗号分隔串', () => {
    expect(coinsToStr(['BTC-USDT', 'ETH-USDT'])).toBe('BTC-USDT, ETH-USDT');
  });
  it('undefined/空数组返回空串', () => {
    expect(coinsToStr(undefined)).toBe('');
    expect(coinsToStr([])).toBe('');
  });
});
