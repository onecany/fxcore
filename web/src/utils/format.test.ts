// 格式化纯函数测试。
import { describe, it, expect } from 'vitest';
import { fmtUsd, fmtPct, fmtDur, safeNum } from './format';

describe('fmtUsd', () => {
  it('正常数值带千分位', () => {
    expect(fmtUsd(10453.28)).toBe('10,453.28');
  });
  it('signed 模式带符号', () => {
    expect(fmtUsd(1284.5, true)).toBe('+1,284.50');
    expect(fmtUsd(-356.2, true)).toBe('-356.20');
  });
  it('非 number/NaN 返回 -（不白屏兜底）', () => {
    expect(fmtUsd(undefined)).toBe('-');
    expect(fmtUsd(null)).toBe('-');
    expect(fmtUsd(Number.NaN)).toBe('-');
  });
});

describe('fmtPct', () => {
  it('比率转百分比一位小数', () => {
    expect(fmtPct(0.62)).toBe('62.0%');
    expect(fmtPct(0.137)).toBe('13.7%');
  });
  it('非 number 返回 -', () => {
    expect(fmtPct(undefined)).toBe('-');
  });
});

describe('fmtDur', () => {
  it('分钟/小时/天数分级', () => {
    expect(fmtDur('2026-08-10T11:00:00Z', '2026-08-10T11:45:00Z')).toBe('45m');
    expect(fmtDur('2026-08-10T11:00:00Z', '2026-08-10T12:20:00Z')).toBe('1.3h');
    expect(fmtDur('2026-08-01T00:00:00Z', '2026-08-04T00:00:00Z')).toBe('3.0d');
  });
  it('缺时间/负时长返回 -', () => {
    expect(fmtDur(undefined, 'x')).toBe('-');
    expect(fmtDur('2026-08-10T12:00:00Z', '2026-08-10T11:00:00Z')).toBe('-');
  });
});

describe('safeNum', () => {
  it('仅接受有限 number', () => {
    expect(safeNum(3.14)).toBe(3.14);
    expect(safeNum(undefined)).toBeUndefined();
    expect(safeNum('x')).toBeUndefined();
    expect(safeNum(Number.NaN)).toBeUndefined();
  });
});
