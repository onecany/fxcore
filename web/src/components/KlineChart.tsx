// K 线图表（lightweight-charts v5，Canvas 渲染）。
// 主题契约：canvas 不支持 CSS 变量（fillStyle 解析不了 var()），
// 颜色从设计令牌解析为实际色值；MutationObserver 监听 data-theme 变化重解析（深浅色切换即时生效）。
import { useEffect, useRef } from 'react';
import { createChart, CandlestickSeries, HistogramSeries, ColorType, LineStyle } from 'lightweight-charts';
import type { IChartApi, ISeriesApi, UTCTimestamp } from 'lightweight-charts';
import type { KlineDTO } from '../api/v1/types/contract';

/** 解析 CSS 变量为实际色值（canvas 不支持 var()） */
function cssVar(name: string, fallback: string): string {
  if (typeof document === 'undefined') return fallback;
  const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  return v || fallback;
}

/** 从当前主题令牌解析全套图表配色 */
function resolvePalette() {
  return {
    up: cssVar('--fxcore-up', '#34d399'),
    down: cssVar('--fxcore-down', '#fb7185'),
    text: cssVar('--fxcore-text-dim', '#8fa3bf'),
    grid: cssVar('--fxcore-bg-grid', 'rgba(240,185,11,0.035)'),
    border: cssVar('--fxcore-panel-border', 'rgba(240,185,11,0.16)'),
    accent: cssVar('--fxcore-gold', '#f0b90b'),
  };
}

export default function KlineChart({ data, height = 320 }: { data: KlineDTO[]; height?: number }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const candleRef = useRef<ISeriesApi<'Candlestick'> | null>(null);
  const volumeRef = useRef<ISeriesApi<'Histogram'> | null>(null);
  const dataRef = useRef<KlineDTO[]>([]);
  // 同步当前数据给主题刷新（refs 不允许在 render 期写入）
  useEffect(() => { dataRef.current = data; }, [data]);

  // 创建/销毁 chart（仅一次）
  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const p = resolvePalette();

    const chart = createChart(el, {
      autoSize: true,
      height,
      layout: {
        background: { type: ColorType.Solid, color: 'transparent' },
        textColor: p.text,
        fontFamily: cssVar('--fxcore-font-mono', 'ui-monospace, Menlo, monospace'),
        fontSize: 11,
      },
      grid: {
        vertLines: { color: p.grid },
        horzLines: { color: p.grid },
      },
      rightPriceScale: { borderColor: p.border },
      timeScale: {
        borderColor: p.border,
        timeVisible: true,
        secondsVisible: false,
        rightOffset: 4,
      },
      crosshair: {
        vertLine: { color: p.accent, width: 1, style: LineStyle.Dashed, labelBackgroundColor: p.accent },
        horzLine: { color: p.accent, width: 1, style: LineStyle.Dashed, labelBackgroundColor: p.accent },
      },
    });

    const candles = chart.addSeries(CandlestickSeries, {
      upColor: p.up,
      downColor: p.down,
      borderUpColor: p.up,
      borderDownColor: p.down,
      wickUpColor: p.up,
      wickDownColor: p.down,
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

  // 主题切换响应：MutationObserver 监听 data-theme，重解析令牌并 applyOptions
  useEffect(() => {
    const chart = chartRef.current;
    const candles = candleRef.current;
    if (!chart || !candles) return;
    const refreshTheme = () => {
      const p = resolvePalette();
      chart.applyOptions({
        layout: { textColor: p.text },
        grid: { vertLines: { color: p.grid }, horzLines: { color: p.grid } },
        rightPriceScale: { borderColor: p.border },
        timeScale: { borderColor: p.border },
        crosshair: {
          vertLine: { color: p.accent, labelBackgroundColor: p.accent },
          horzLine: { color: p.accent, labelBackgroundColor: p.accent },
        },
      });
      candles.applyOptions({
        upColor: p.up,
        downColor: p.down,
        borderUpColor: p.up,
        borderDownColor: p.down,
        wickUpColor: p.up,
        wickDownColor: p.down,
      });
      // 成交量颜色跟随涨跌，主题切换后重算
      const rows = dataRef.current;
      if (rows.length > 0 && volumeRef.current) {
        volumeRef.current.setData(
          rows.map((k) => ({
            time: k.timestamp as UTCTimestamp,
            value: k.volume,
            color: k.close >= k.open ? p.up : p.down,
          })),
        );
      }
    };
    const obs = new MutationObserver((muts) => {
      if (muts.some((m) => m.attributeName === 'data-theme')) refreshTheme();
    });
    obs.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
    return () => obs.disconnect();
  }, []);

  // 数据更新（setData 全量替换；空数组清空）
  // 注意：数据源链可能返回降序（实测 200 根全逆序），lightweight-charts setData 要求严格按时间升序，
  // 乱序会抛错导致渲染崩溃——先排序再喂。
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
    const p = resolvePalette();
    volumeRef.current?.setData(
      sorted.map((k) => ({
        time: k.timestamp as UTCTimestamp,
        value: k.volume,
        color: k.close >= k.open ? p.up : p.down,
      })),
    );
    if (bars.length > 0) chartRef.current?.timeScale().fitContent();
  }, [data]);

  return <div ref={containerRef} style={{ width: '100%', height }} />;
}
