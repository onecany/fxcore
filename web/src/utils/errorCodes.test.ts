import { describe, it, expect } from 'vitest';
import { errMsg, getErrorMessage } from './errorCodes';

describe('errMsg', () => {
  it('Error 对象取 message 并剥 Error: 前缀', () => {
    expect(errMsg(new Error('无法连接服务器，请检查网络或稍后重试'))).toBe('无法连接服务器，请检查网络或稍后重试');
    expect(errMsg(new Error('Error: Network Error'))).toBe('Network Error');
  });

  it('AppError（Error 子类）同样处理', () => {
    // AppError 的 message 就是后端/兜底消息
    expect(errMsg(new Error('服务器暂时不可用，请稍后重试'))).toBe('服务器暂时不可用，请稍后重试');
  });

  it('字符串原样剥前缀', () => {
    expect(errMsg('请求超时，请检查网络后重试')).toBe('请求超时，请检查网络后重试');
    expect(errMsg('Error: 服务器内部错误')).toBe('服务器内部错误');
  });

  it('空值/未知类型给通用兜底文案（不抛异常）', () => {
    expect(errMsg(null)).toBe('操作失败，请稍后重试');
    expect(errMsg(undefined)).toBe('操作失败，请稍后重试');
    expect(errMsg('')).toBe('操作失败，请稍后重试');
    expect(errMsg(42 as unknown)).toBe('操作失败，请稍后重试');
  });
});

describe('getErrorMessage', () => {
  it('未知错误码回退 fallback', () => {
    expect(getErrorMessage(-1, 'Network Error')).toBe('Network Error');
    expect(getErrorMessage(9999)).toBe('请求失败');
  });
});
