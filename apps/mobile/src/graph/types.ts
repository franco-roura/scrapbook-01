// Shapes returned by GET /v1/graph and GET /v1/items/{id}/neighbors.
export type GraphNode = {
  id: string;
  kind: 'item' | 'category';
  label: string;
  platform?: string;
  categoryId?: string;
  degree: number;
};

export type GraphLink = {
  source: string;
  target: string;
  kind: 'similar' | 'category';
  weight: number;
};

export type GraphData = { nodes: GraphNode[]; links: GraphLink[]; truncated: boolean };
