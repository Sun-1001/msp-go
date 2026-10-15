import type { NoteNetwork } from './types';
export interface Point { x: number; y: number }

// Deterministic breadth-first layers keep connected notes together without a
// continuously moving simulation. Pack disconnected components into shelves so
// a collection of unlinked notes does not collapse into one very tall column.
export function layoutNetwork(network: NoteNetwork): Record<string, Point> {
  const adjacent = new Map(network.nodes.map((node) => [node.id, new Set<string>()]));
  network.links.forEach(({ source_id, target_id }) => {
    adjacent.get(source_id)?.add(target_id);
    adjacent.get(target_id)?.add(source_id);
  });
  const order = [...adjacent.keys()].sort((a, b) => (adjacent.get(b)?.size ?? 0) - (adjacent.get(a)?.size ?? 0) || a.localeCompare(b));
  const seen = new Set<string>();
  const result: Record<string, Point> = {};
  const shelfWidth = Math.max(800, Math.ceil(Math.sqrt(network.nodes.length)) * 200);
  let shelfX = 0;
  let shelfY = 0;
  let shelfHeight = 0;
  for (const root of order) {
    if (seen.has(root)) continue;
    let layer = [root];
    seen.add(root);
    let depth = 0;
    let height = 90;
    const component: Record<string, Point> = {};
    while (layer.length) {
      const next: string[] = [];
      layer.forEach((id, row) => {
        component[id] = { x: depth * 200, y: row * 90 };
        for (const neighbor of [...(adjacent.get(id) ?? [])].sort()) {
          if (adjacent.has(neighbor) && !seen.has(neighbor)) { seen.add(neighbor); next.push(neighbor); }
        }
      });
      height = Math.max(height, layer.length * 90);
      layer = next;
      depth += 1;
    }
    const width = depth * 200;
    if (shelfX && shelfX + width > shelfWidth) {
      shelfY += shelfHeight + 90;
      shelfX = 0;
      shelfHeight = 0;
    }
    Object.entries(component).forEach(([id, point]) => {
      result[id] = { x: point.x + shelfX, y: point.y + shelfY };
    });
    shelfX += width;
    shelfHeight = Math.max(shelfHeight, height);
  }
  return result;
}

export function internalLink(id: string, title: string): string {
  // Entities keep titles containing brackets, backticks or a trailing backslash
  // readable without turning the link label into Markdown syntax.
  const label = title.replace(/[&<>`\\[\]]/g, (character) => `&#${character.charCodeAt(0)};`).replace(/[\r\n]/g, ' ');
  return `[${label}](/notes/${id})`;
}
