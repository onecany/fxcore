// K 线图表（lightweight-charts v5，Canvas 渲染）。
// 主题契约：canvas 不支持 CSS 变量（fillStyle 解析不了 var()），
// 颜色在挂载时从 :root 设计令牌解析为实际色值，绿涨红跌与全局一致。
import { useEffect, useRef } from 'react';
import { createChart, CandlestickSeries, HistogramSeries, ColorType, LineStyle } from 'lightweight-charts';
import type { IChartApi, ISeriesApi, UTCTimestamp } from 'lightweight-charts';
import type { KlineDTO } from '../api/v1/types/contract';

/** 解析 :root CSS 变量为实际色值（canvas 不支持 var()） */
function cssVar(name: string, fallback: string): string {
  if (typeof document === 'undefined') return fallback;
  const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  return v || fallback;
}

export default function KlineChart({ data, height = 320 }: { data: KlineDTO[]; height?: number }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const candleRef = useRef<ISeriesApi<'Candlestick'> | null>(null);
  const volumeRef = useRef<ISeriesApi<'Histogram'> | null>(null);

  // 创建/销毁 chart（仅一次；主题固定深色，无需响应切换）
  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const up = cssVar('--fxcore-up', '#34d399');
    const down = cssVar('--fxcore-down', '#fb7185');
    const text = cssVar('--fxcore-text-dim', '#8fa3bf');
    const grid = cssVar('--fxcore-bg-grid', 'rgba(240,185,11,0.035)');
    const border = cssVar('--fxcore-panel-border', 'rgba(240,185,11,0.16)');
    const accent = cssVar('--fxcore-gold', '#f0b90b');

    const chart = createChart(el, {
      autoSize: true,
      height,
      layout: {
        background: { type: ColorType.Solid, color: 'transparent' },
        textColor: text,
        fontFamily: cssVar('--fxcore-font-mono', 'ui-monospace, Menlo, monospace'),
        fontSize: 11,
      },
      grid: {
        vertLines: { color: grid },
        horzLines: { color: grid },
      },
      rightPriceScale: { borderColor: border },
      timeScale: {
        borderColor: border,
        timeVisible: true,
        secondsVisible: false,
        rightOffset: 4,
      },
      crosshair: {
        vertLine: { color: accent, width: 1, style: LineStyle.Dashed, labelBackgroundColor: accent },
        horzLine: { color: accent, width: 1, style: LineStyle.Dashed, labelBackgroundColor: accent },
      },
    });

    const candles = chart.addSeries(CandlestickSeries, {
      upColor: up,
      downColor: down,
      borderUpColor: up,
      borderDownColor: down,
      wickUpColor: up,
      wickDownColor: down,
    });
    // 成交量 overlay（底部 18% 区域，颜色跟随涨跌）
    const volume = chart.addSeries(HistogramSeries, {
      priceFormat: { type: 'volume' },
      priceScaleId: 'vol',
      lastValueVisible: false,
      priceLineVisible: false,
    });
    chart.priceScale('vol').applyOptions({ scaleMargins: { top: 0.82, bottom: 0 } });

    chartRef.current = chart;
    candleRef.current = candles;
    volumeRef.current = volume;
    return () => {
      chart.remove();
      chartRef.current = null;
      candleRef.current = null;
      volumeRef.current = null;
    };
  }, [height]);

  // 数据更新（setData 全量替换；空数组清空）
  // 注意：数据源链（hyperliquid→okx→coinank）可能返回降序（实测 200 根全逆序），
  // lightweight-charts setData 要求严格按时间升序，乱序会抛错导致渲染崩溃——先排序再喂。
  useEffect(() => {
    const candles = candleRef.current;
    if (!candles) return;
    const sorted = [...data].sort((a, b) => a.timestamp - b.timestamp);
    const bars = sorted.map((k) => ({
      time: k.timestamp as UTCTimestamp,
      open: k.open,
      high: k.high,
      low: k.low,
      close: k.close,
    }));
    candles.setData(bars);
    volumeRef.current?.setData(
      sorted.map((k) => ({
        time: k.timestamp as UTCTimestamp,
        value: k.volume,
        color: k.close >= k.open ? cssVar('--fxcore-up', '#34d399') : cssVar('--fxcore-down', '#fb7185'),
      })),
    );
    if (bars.length > 0) chartRef.current?.timeScale().fitContent();
  }, [data]);

  return <div ref={containerRef} style={{ width: '100%', height }} />;
}
