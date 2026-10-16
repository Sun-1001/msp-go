import { useEffect, useMemo, useRef, useState } from 'react';
import { Focus, Minus, Plus, RotateCcw } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { layoutNetwork, type Point } from './layout';
import type { NoteNetwork as Network } from './types';

interface Props { network: Network; selected?: string; search: string; onOpen: (id: string) => void }
export function NoteNetwork({ network, selected, search, onOpen }: Props) {
  const initial = useMemo(() => layoutNetwork(network), [network]);
  const [moved, setMoved] = useState<Record<string, Point>>({});
  const points = { ...initial, ...moved };
  const bounds = Object.values(initial);
  const width = Math.max(800, ...bounds.map((p) => p.x + 240));
  const height = Math.max(500, ...bounds.map((p) => p.y + 160));
  const [view, setView] = useState<{ x: number; y: number; zoom: number }>({ x: 0, y: 0, zoom: 1 });
  const svg = useRef<SVGSVGElement>(null);
  const drag = useRef<{ id?: string; x: number; y: number; startX: number; startY: number; moved: boolean } | null>(null);
  const visibleWidth = width / view.zoom;
  const visibleHeight = height / view.zoom;
  const center = { x: width / 2 + view.x, y: height / 2 + view.y };
  const zoom = (factor: number) => setView((v) => ({ ...v, zoom: Math.max(0.25, Math.min(20, v.zoom * factor)) }));
  useEffect(() => {
    const element = svg.current;
    const wheel = (event: WheelEvent) => {
      if (event.ctrlKey || event.metaKey) {
        event.preventDefault();
        setView((v) => ({ ...v, zoom: Math.max(0.25, Math.min(20, v.zoom * (event.deltaY > 0 ? 0.9 : 1.1))) }));
      }
    };
    element?.addEventListener('wheel', wheel, { passive: false });
    return () => element?.removeEventListener('wheel', wheel);
  }, [network.nodes.length]);
  const focus = () => {
    if (!selected || !points[selected]) return;
    const ids = new Set([selected]);
    network.links.forEach((link) => {
      if (link.source_id === selected) ids.add(link.target_id);
      if (link.target_id === selected) ids.add(link.source_id);
    });
    const group = [...ids].map((id) => points[id]).filter(Boolean);
    const left = Math.min(...group.map((p) => p.x));
    const right = Math.max(...group.map((p) => p.x));
    const top = Math.min(...group.map((p) => p.y));
    const bottom = Math.max(...group.map((p) => p.y));
    setView({ x: (left + right) / 2 + 120 - width / 2, y: (top + bottom) / 2 + 70 - height / 2, zoom: Math.max(0.25, Math.min(20, width / Math.max(500, right - left + 260), height / Math.max(300, bottom - top + 160))) });
  };
  const matching = new Set(network.nodes.filter((n) => n.title.toLocaleLowerCase().includes(search.toLocaleLowerCase())).map((n) => n.id));
  return <section className="overflow-hidden rounded-lg border border-surface-200 bg-surface-50 dark:border-surface-700 dark:bg-surface-950" aria-label="个人知识关联网络">
    <div className="flex flex-wrap items-center gap-2 border-b border-surface-200 p-3 dark:border-surface-700">
      <Button variant="outline" size="sm" aria-label="放大" onClick={() => zoom(1.3)}><Plus className="h-4 w-4" /></Button>
      <Button variant="outline" size="sm" aria-label="缩小" onClick={() => zoom(1 / 1.3)}><Minus className="h-4 w-4" /></Button>
      <Button variant="outline" size="sm" onClick={() => { setView({ x: 0, y: 0, zoom: 1 }); setMoved({}); }}><RotateCcw className="mr-1 h-4 w-4" />重置视图</Button>
      <Button variant="outline" size="sm" disabled={!selected || !points[selected]} onClick={focus}><Focus className="mr-1 h-4 w-4" />聚焦当前笔记</Button>
      <span className="text-xs text-surface-500">{network.nodes.length} 篇笔记 · {network.links.length} 条链接</span>
    </div>
    {network.nodes.length === 0 ? <p className="p-16 text-center text-surface-500">网络还是空的。创建第一篇笔记，再添加内部链接。</p> : <>
      {search && matching.size === 0 && <p role="status" className="px-4 pt-4 text-sm text-surface-500">没有匹配的笔记标题，清空搜索可查看全部。</p>}
      <svg ref={svg} className="h-[520px] w-full touch-none select-none" viewBox={`${center.x - visibleWidth / 2} ${center.y - visibleHeight / 2} ${visibleWidth} ${visibleHeight}`} role="group" aria-label="拖动笔记调整位置，拖动空白平移，点击笔记打开"
        onPointerDown={(event) => {
          if (event.button !== 0) return;
          const target = event.target instanceof Element ? event.target.closest('[data-note-id]')?.getAttribute('data-note-id') : undefined;
          drag.current = { id: target ?? undefined, x: event.clientX, y: event.clientY, startX: event.clientX, startY: event.clientY, moved: false };
          event.currentTarget.setPointerCapture(event.pointerId);
        }}
        onPointerMove={(event) => {
          const state = drag.current; const matrix = svg.current?.getScreenCTM();
          if (!state || !matrix) return;
          const dx = (event.clientX - state.x) / matrix.a; const dy = (event.clientY - state.y) / matrix.d;
          state.moved ||= Math.hypot(event.clientX - state.startX, event.clientY - state.startY) > 4;
          if (state.moved) {
            if (state.id) { const id = state.id; const p = points[id]; if (p) setMoved((old) => ({ ...old, [id]: { x: p.x + dx, y: p.y + dy } })); }
            else setView((old) => ({ ...old, x: old.x - dx, y: old.y - dy }));
          }
          state.x = event.clientX; state.y = event.clientY;
        }}
        onPointerUp={() => { const state = drag.current; drag.current = null; if (state?.id && !state.moved) onOpen(state.id); }}
        onPointerCancel={() => { drag.current = null; }}>
        <defs><marker id="note-arrow" viewBox="0 0 10 10" refX="10" refY="5" markerWidth="5" markerHeight="5" orient="auto-start-reverse"><path d="M 0 0 L 10 5 L 0 10 z" fill="currentColor" /></marker></defs>
        <g transform="translate(120 70)">
          {network.links.map((link) => {
            const a = points[link.source_id]; const b = points[link.target_id];
            if (!a || !b) return null;
            const distance = Math.max(1, Math.hypot(b.x - a.x, b.y - a.y));
            return <line key={`${link.source_id}:${link.target_id}`} x1={a.x} y1={a.y} x2={b.x - (b.x - a.x) * 14 / distance} y2={b.y - (b.y - a.y) * 14 / distance} stroke="currentColor" strokeWidth="1.5" markerEnd="url(#note-arrow)" className="text-surface-400 dark:text-surface-600" opacity={search && (!matching.has(link.source_id) && !matching.has(link.target_id)) ? 0.15 : 0.7} />;
          })}
          {network.nodes.map((node) => {
            const p = points[node.id]; if (!p) return null;
            return <g key={node.id} transform={`translate(${p.x} ${p.y})`} data-note-id={node.id} role="button" tabIndex={0} aria-label={`打开笔记：${node.title}`} className="cursor-pointer outline-none focus:stroke-primary-500" opacity={search && !matching.has(node.id) ? 0.25 : 1} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); onOpen(node.id); } }}>
              <title>{node.title}</title>
              <rect x="-90" y="-20" width="180" height="64" fill="transparent" stroke="none" />
              <circle r={node.id === selected ? 14 : 10} className={node.id === selected ? 'fill-primary-500' : 'fill-surface-400 dark:fill-surface-500'} strokeWidth="3" />
              <text y="32" textAnchor="middle" className="fill-surface-800 text-[13px] dark:fill-surface-200" stroke="none">{node.title.length > 14 ? `${node.title.slice(0, 14)}…` : node.title}</text>
            </g>;
          })}
        </g>
      </svg>
      <p className="px-4 pb-3 text-xs text-surface-500">点击打开 · 拖动调整 · 拖动空白平移 · Ctrl + 滚轮缩放 · 箭头指向被引用的笔记</p>
    </>}
  </section>;
}
