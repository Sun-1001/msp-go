import { useEffect, useRef, useState } from 'react';
import { useBeforeUnload, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import ReactMarkdown from 'react-markdown';
import { BookOpen, FilePlus2, Network, Save, Search, Trash2, Upload, Loader2, Link2, Download } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { RequestErrorNotice } from '@/components/feedback';
import { MARKDOWN_COMPONENTS, REMARK_PLUGINS, REHYPE_PLUGINS } from '@/components/chat/markdownRenderConfig';
import { toAppError, type AppError } from '@/libs/http/apiClient';
import { noteService } from './service';
import { NoteNetwork } from './NoteNetwork';
import { internalLink } from './layout';
import type { Note, NoteInput, NoteMaterial, NoteNetwork as NetworkData, NoteSummary } from './types';

const emptyInput: NoteInput = { title: '', body: '', material_id: null, source_info: '', revision: 0 };
const emptyNetwork: NetworkData = { nodes: [], links: [] };
const fieldClass = 'w-full rounded-md border border-surface-200 bg-white p-3 text-sm text-surface-900 focus:outline-none focus:ring-2 focus:ring-primary-500 disabled:opacity-60 dark:border-surface-700 dark:bg-surface-900 dark:text-surface-100';
const inputOf = (note: Note): NoteInput => ({ title: note.title, body: note.body, material_id: note.material_id, source_info: note.source_info, revision: note.revision });

export function NoteWorkspace() {
  const { noteId } = useParams();
  const [params, setParams] = useSearchParams();
  const navigate = useNavigate();
  const graphView = params.get('view') === 'network';
  const localView = params.get('scope') === 'local' && !!noteId;
  const [network, setNetwork] = useState<NetworkData>(emptyNetwork);
  const [localNetwork, setLocalNetwork] = useState<NetworkData>(emptyNetwork);
  const [notes, setNotes] = useState<NoteSummary[]>([]);
  const [materials, setMaterials] = useState<NoteMaterial[]>([]);
  const [note, setNote] = useState<Note | null>(null);
  const [draft, setDraft] = useState<NoteInput>(emptyInput);
  const [search, setSearch] = useState('');
  const [linkTarget, setLinkTarget] = useState('');
  const [preview, setPreview] = useState(true);
  const [loading, setLoading] = useState(true);
  const [searching, setSearching] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<AppError | null>(null);
  const [readError, setReadError] = useState<AppError | null>(null);
  const [searchError, setSearchError] = useState<AppError | null>(null);
  const [status, setStatus] = useState('');
  const [version, setVersion] = useState(0);
  const [searchVersion, setSearchVersion] = useState(0);
  const uploadInput = useRef<HTMLInputElement>(null);
  const mutation = useRef<AbortController | null>(null);
  const dirty = JSON.stringify(draft) !== JSON.stringify(note ? inputOf(note) : emptyInput);
  useBeforeUnload((event) => { if (dirty) { event.preventDefault(); event.returnValue = ''; } });
  useEffect(() => () => mutation.current?.abort(), []);

  useEffect(() => {
    const controller = new AbortController();
    const load = async () => {
      setLoading(true); setReadError(null); setError(null);
      try {
        const [graph, files, selected, local] = await Promise.all([
          noteService.network(undefined, controller.signal), noteService.materials(controller.signal),
          noteId ? noteService.get(noteId, controller.signal) : Promise.resolve(null),
          noteId ? noteService.network(noteId, controller.signal) : Promise.resolve(emptyNetwork),
        ]);
        if (controller.signal.aborted) return;
        setNetwork(graph); setMaterials(files); setNote(selected); setDraft(selected ? inputOf(selected) : emptyInput); setLocalNetwork(local); setLinkTarget('');
      } catch (err) { if (!controller.signal.aborted) setReadError(toAppError(err, '加载笔记失败')); }
      finally { if (!controller.signal.aborted) setLoading(false); }
    };
    void load();
    return () => controller.abort();
  }, [noteId, version]);

  useEffect(() => {
    const controller = new AbortController();
    const timer = setTimeout(() => {
      setSearching(true); setSearchError(null);
      void noteService.list(search, controller.signal).then((items) => { if (!controller.signal.aborted) setNotes(items); })
        .catch((err: unknown) => { if (!controller.signal.aborted) { setSearchError(toAppError(err, '搜索笔记失败')); setNotes([]); } })
        .finally(() => { if (!controller.signal.aborted) setSearching(false); });
    }, 200);
    return () => { clearTimeout(timer); controller.abort(); };
  }, [search, version, noteId, searchVersion]);

  const open = (id?: string) => {
    if (busy || (dirty && !window.confirm('当前笔记尚未保存，确定放弃修改并打开其他笔记吗？'))) return;
    setStatus(''); setPreview(true);
    if (!id && !noteId) { setDraft(emptyInput); setNote(null); setPreview(false); }
    navigate(id ? `/notes/${id}` : '/notes');
  };
  const run = async (action: (signal: AbortSignal) => Promise<void>) => {
    if (busy) return;
    const controller = new AbortController(); mutation.current = controller;
    setBusy(true); setError(null); setStatus('');
    try { await action(controller.signal); }
    catch (err) { if (!controller.signal.aborted) setError(toAppError(err, '操作失败，修改仍保留在编辑区')); }
    finally { if (!controller.signal.aborted) setBusy(false); }
  };
  const save = () => run(async (signal) => {
    const saved = await noteService.save(noteId, draft, signal);
    if (signal.aborted) return;
    setNote(saved); setDraft(inputOf(saved)); setStatus('笔记已保存，关联网络已更新'); setVersion((v) => v + 1);
    if (!noteId) navigate(`/notes/${saved.id}`, { replace: true });
  });
  const remove = () => {
    if (!note || !window.confirm(`删除“${note.title}”？关联链接会同步移除，来源材料仍保留。`)) return;
    void run(async (signal) => {
      await noteService.remove(note.id, note.revision, signal);
      if (signal.aborted) return;
      setNote(null); setDraft(emptyInput); setVersion((v) => v + 1); navigate('/notes', { replace: true }); setStatus('笔记已删除');
    });
  };
  const upload = (file: File) => run(async (signal) => {
    const material = await noteService.upload(file, signal);
    let body = draft.body;
    if (/\.(md|txt)$/i.test(file.name) && !body) {
      const text = await file.text();
      if ([...text].length <= 200000) body = text;
    }
    if (signal.aborted) return;
    setMaterials((items) => [material, ...items]);
    setDraft((old) => ({ ...old, material_id: material.id, title: old.title || file.name.replace(/\.[^.]+$/, '').slice(0, 200), body }));
    setPreview(false); setStatus('材料已上传并关联，请保存笔记');
  });
  const linkedIDs = new Set(network.links.filter((l) => l.source_id === noteId || l.target_id === noteId).map((l) => l.source_id === noteId ? l.target_id : l.source_id));
  const linked = network.nodes.filter((n) => linkedIDs.has(n.id));
  const material = materials.find((m) => m.id === draft.material_id);
  const reload = () => { if (!dirty || window.confirm('重新加载将丢弃未保存修改，是否继续？')) setVersion((v) => v + 1); };

  return <div className="mx-auto max-w-[1600px] px-4 py-6 sm:px-6">
    <header className="mb-6 flex flex-wrap items-center justify-between gap-4">
      <div><h1 className="text-2xl font-bold text-surface-900 dark:text-surface-100">知识关联</h1><p className="mt-1 text-sm text-surface-500">记录所学，用链接连接自己的思考。</p></div>
      <div className="flex gap-2">
        <Button variant="outline" disabled={busy} onClick={() => open()}><FilePlus2 className="mr-2 h-4 w-4" />新建笔记</Button>
        <Button variant={graphView ? 'primary' : 'outline'} onClick={() => setParams(graphView ? {} : { view: 'network' })}>{graphView ? <BookOpen className="mr-2 h-4 w-4" /> : <Network className="mr-2 h-4 w-4" />}{graphView ? '返回笔记' : '查看关联网络'}</Button>
      </div>
    </header>
    <div className="grid items-start gap-5 lg:grid-cols-[260px_minmax(0,1fr)]">
      <aside className="rounded-lg border border-surface-200 bg-white p-4 dark:border-surface-700 dark:bg-surface-900">
        <label className="mb-2 flex items-center gap-2 text-sm font-medium" htmlFor="note-search"><Search className="h-4 w-4" />我的笔记</label>
        <Input id="note-search" value={search} maxLength={200} placeholder="搜索标题或正文" onChange={(e) => setSearch(e.target.value)} />
        {searchError && <RequestErrorNotice error={searchError} onRetry={() => setSearchVersion((v) => v + 1)} className="mt-3" />}
        <div className="mt-3 max-h-60 space-y-1 overflow-y-auto lg:max-h-[650px]" aria-busy={searching}>
          {notes.map((item) => <button key={item.id} type="button" disabled={busy} onClick={() => open(item.id)} className={`w-full rounded-md px-3 py-3 text-left text-sm transition-colors ${noteId === item.id ? 'bg-primary-50 text-primary-700 dark:bg-primary-950 dark:text-primary-300' : 'hover:bg-surface-100 dark:hover:bg-surface-800'}`} aria-current={noteId === item.id ? 'page' : undefined}><span className="block truncate font-medium">{item.title}</span><span className="mt-1 block text-xs text-surface-500">{new Date(item.updated_at).toLocaleDateString('zh-CN')}</span></button>)}
          {!searching && !searchError && notes.length === 0 && <p className="py-8 text-center text-sm text-surface-500">{search ? '没有匹配的笔记' : '从第一篇学习笔记开始'}</p>}
          {searching && <p role="status" className="p-3 text-xs text-surface-500">正在搜索…</p>}
        </div>
      </aside>
      <main className="min-w-0 space-y-4">
        {status && <p role="status" className="rounded-md bg-primary-50 px-4 py-3 text-sm text-primary-700 dark:bg-primary-950 dark:text-primary-300">{status}</p>}
        {error && <RequestErrorNotice error={error} onRefresh={reload} />}
        {readError ? <RequestErrorNotice error={readError} onRetry={reload} onRefresh={reload} /> : loading ? <div role="status" className="flex items-center justify-center gap-2 p-20"><Loader2 className="h-5 w-5 animate-spin" />正在加载笔记…</div> : graphView ? <>
          <div className="flex flex-wrap items-center gap-2"><Button size="sm" variant={!localView ? 'primary' : 'outline'} onClick={() => setParams({ view: 'network' })}>完整个人网络</Button><Button size="sm" variant={localView ? 'primary' : 'outline'} disabled={!noteId} onClick={() => setParams({ view: 'network', scope: 'local' })}>当前笔记及关联</Button><span className="text-sm text-surface-500">{note?.title}</span></div>
          <NoteNetwork key={`${noteId ?? 'all'}:${localView}:${version}`} network={localView ? localNetwork : network} selected={noteId} search={search} onOpen={open} />
        </> : <section className="rounded-lg border border-surface-200 bg-white dark:border-surface-700 dark:bg-surface-900">
          <div className="flex flex-wrap items-center justify-between gap-3 border-b border-surface-200 p-4 dark:border-surface-700">
            <span className="text-sm text-surface-500">{note ? `更新于 ${new Date(note.updated_at).toLocaleString('zh-CN')}` : '新笔记'}{dirty ? ' · 未保存' : ''}</span>
            <div className="flex gap-2"><Button size="sm" variant="outline" onClick={() => setPreview((v) => !v)}>{preview ? '编辑' : '预览'}</Button><Button size="sm" disabled={busy || !draft.title.trim() || !dirty} onClick={() => void save()}><Save className="mr-1 h-4 w-4" />{busy ? '处理中…' : '保存'}</Button>{note && <Button size="sm" variant="ghost" disabled={busy} onClick={remove} aria-label="删除笔记"><Trash2 className="h-4 w-4" /></Button>}</div>
          </div>
          <div className="space-y-4 p-4 sm:p-6">
            <label className="block space-y-2 text-sm font-medium"><span>标题</span><Input value={draft.title} maxLength={200} disabled={busy} onChange={(e) => setDraft({ ...draft, title: e.target.value })} placeholder="例如：导数与函数变化率" /></label>
            {preview && draft.body ? <article className="min-h-72 break-words text-surface-800 dark:text-surface-200"><ReactMarkdown remarkPlugins={REMARK_PLUGINS} rehypePlugins={REHYPE_PLUGINS} components={{ ...MARKDOWN_COMPONENTS, a: ({ href, children }) => {
              const match = href?.match(/^\/notes\/([0-9a-f-]{36})$/);
              if (match) return network.nodes.some((n) => n.id === match[1]) ? <a href={href} className="rounded bg-primary-50 px-1 text-primary-700 underline decoration-primary-300 dark:bg-primary-950 dark:text-primary-300" onClick={(e) => { e.preventDefault(); open(match[1]); }}>{children}</a> : <span title="笔记不可用" className="text-surface-400">{children}（不可用）</span>;
              return <a href={href} target="_blank" rel="noopener noreferrer" className="text-primary-600 underline">{children}</a>;
            } }}>{draft.body}</ReactMarkdown></article> : <label className="block space-y-2 text-sm font-medium"><span>正文 · Markdown</span><textarea className={`${fieldClass} min-h-72 resize-y font-mono leading-7`} value={draft.body} maxLength={200000} disabled={busy} onChange={(e) => setDraft({ ...draft, body: e.target.value })} placeholder="写下理解、公式和疑问。通过下面的选择框插入笔记链接。" /></label>}
            <div className="flex flex-wrap gap-2"><select aria-label="选择要链接的笔记" className={`${fieldClass} min-w-0 flex-1`} value={linkTarget} disabled={busy} onChange={(e) => setLinkTarget(e.target.value)}><option value="">选择一篇笔记建立链接</option>{network.nodes.filter((n) => n.id !== noteId).map((n) => <option key={n.id} value={n.id}>{n.title}</option>)}</select><Button variant="outline" disabled={busy || !linkTarget} onClick={() => { const target = network.nodes.find((n) => n.id === linkTarget); if (target) { setDraft({ ...draft, body: `${draft.body}\n\n${internalLink(target.id, target.title)}` }); setPreview(false); setLinkTarget(''); } }}><Link2 className="mr-1 h-4 w-4" />插入链接</Button></div>
            <p className="text-xs text-surface-500">保存后自动生成关联；删除正文中的链接并保存即可解除。改名不会影响按 ID 建立的链接。</p>
            <div className="space-y-3 border-t border-surface-200 pt-4 dark:border-surface-700">
              <label className="block space-y-2 text-sm"><span>来源信息</span><Input value={draft.source_info} maxLength={1000} disabled={busy} onChange={(e) => setDraft({ ...draft, source_info: e.target.value })} placeholder="课程、教材页码或来源说明" /></label>
              <div className="flex flex-wrap gap-2"><select aria-label="关联学习材料" value={draft.material_id ?? ''} disabled={busy} onChange={(e) => setDraft({ ...draft, material_id: e.target.value || null })} className={`${fieldClass} min-w-0 flex-1`}><option value="">不关联材料</option>{materials.map((m) => <option key={m.id} value={m.id}>{m.filename}</option>)}</select><Button variant="outline" disabled={busy} onClick={() => uploadInput.current?.click()}><Upload className="mr-1 h-4 w-4" />上传材料</Button>{material && <Button variant="ghost" disabled={busy} aria-label="下载来源材料" onClick={() => void run((signal) => noteService.download(material, signal))}><Download className="h-4 w-4" /></Button>}</div>
              <input ref={uploadInput} type="file" className="hidden" accept=".pdf,.docx,.txt,.md" onChange={(e) => { const file = e.target.files?.[0]; e.target.value = ''; if (file) void upload(file); }} />
              <p className="text-xs text-surface-500">支持 PDF、DOCX、TXT、Markdown，最大 10 MB。空白正文会导入不超过 20 万字的文本文件，其他材料保留为附件。仅本人可以访问。</p>
            </div>
            {note && <div className="border-t border-surface-200 pt-4 dark:border-surface-700"><div className="mb-3 flex items-center justify-between"><h2 className="font-medium">关联与反向链接 <span className="text-surface-500">{linked.length}</span></h2><Button size="sm" variant="ghost" onClick={() => setParams({ view: 'network', scope: 'local' })}>查看局部网络</Button></div><div className="flex flex-wrap gap-2">{linked.map((n) => <Button key={n.id} size="sm" variant="outline" onClick={() => open(n.id)}>{n.title}</Button>)}{linked.length === 0 && <p className="text-sm text-surface-500">还没有关联，插入内部链接后保存即可建立连接。</p>}</div><p className="mt-4 text-xs text-surface-400">创建于 {new Date(note.created_at).toLocaleString('zh-CN')}</p></div>}
          </div>
        </section>}
      </main>
    </div>
  </div>;
}
