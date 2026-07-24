declare module 'd3-force-3d' {
  export interface SimulationNodeDatum {
    index?: number;
    x?: number;
    y?: number;
    z?: number;
    vx?: number;
    vy?: number;
    vz?: number;
  }

  export interface Force<NodeDatum extends SimulationNodeDatum> {
    (alpha: number): void;
    initialize?: (nodes: NodeDatum[], dimensions?: number) => void;
  }

  export interface ForceRadial<NodeDatum extends SimulationNodeDatum> extends Force<NodeDatum> {
    strength(value: number | ((node: NodeDatum, index: number, nodes: NodeDatum[]) => number)): this;
  }

  export function forceRadial<NodeDatum extends SimulationNodeDatum>(
    radius?: number | ((node: NodeDatum, index: number, nodes: NodeDatum[]) => number),
    x?: number,
    y?: number,
    z?: number,
  ): ForceRadial<NodeDatum>;
}
