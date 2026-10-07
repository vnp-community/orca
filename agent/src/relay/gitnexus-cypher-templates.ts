export type CypherColumnType = 'string' | 'nullableString' | 'int' | 'float' | 'jsonArray'

export type CypherTemplate = {
  id: string
  text: string
  slots: Record<string, 'int' | 'string' | 'stringList' | 'kindList'>
  columns: ReadonlyArray<{ header: string; type: CypherColumnType }>
  freeTextColumn?: true
  verifiedByCr: boolean
}

export const CYPHER_TEMPLATES: Record<string, CypherTemplate> = {
  OV_CLUSTERS: {
    id: 'OV_CLUSTERS',
    text: 'MATCH (c:Community) RETURN c.id, c.symbolCount, c.cohesion, c.keywords, c.label ORDER BY c.symbolCount DESC, c.id LIMIT {{topN}}',
    slots: { topN: 'int' },
    columns: [
      { header: 'c.id', type: 'string' },
      { header: 'c.symbolCount', type: 'int' },
      { header: 'c.cohesion', type: 'float' },
      { header: 'c.keywords', type: 'jsonArray' },
      { header: 'c.label', type: 'string' }
    ],
    freeTextColumn: true,
    verifiedByCr: true
  },
  OV_EDGES: {
    id: 'OV_EDGES',
    text: "MATCH (a)-[r:CodeRelation]->(b) WHERE r.type IN {{kinds}} MATCH (a)-[:CodeRelation {type:'MEMBER_OF'}]->(ca:Community), (b)-[:CodeRelation {type:'MEMBER_OF'}]->(cb:Community) WHERE ca.id <> cb.id RETURN ca.id, cb.id, r.type, count(*) AS w ORDER BY w DESC LIMIT {{maxEdges}}",
    slots: { kinds: 'kindList', maxEdges: 'int' },
    columns: [
      { header: 'ca.id', type: 'string' },
      { header: 'cb.id', type: 'string' },
      { header: 'r.type', type: 'string' },
      { header: 'w', type: 'int' }
    ],
    verifiedByCr: true
  },
  OV_TOPFILES: {
    id: 'OV_TOPFILES',
    text: "MATCH (s)-[m:CodeRelation {type:'MEMBER_OF'}]->(c:Community) WHERE c.id IN {{ids}} RETURN c.id, s.filePath, count(*) AS n",
    slots: { ids: 'stringList' },
    columns: [
      { header: 'c.id', type: 'string' },
      { header: 's.filePath', type: 'string' },
      { header: 'n', type: 'int' }
    ],
    verifiedByCr: true
  },
  OV_COUNT: {
    id: 'OV_COUNT',
    text: 'MATCH (c:Community) RETURN count(*) AS n',
    slots: {},
    columns: [{ header: 'n', type: 'int' }],
    verifiedByCr: true
  },
  PR_LIST: {
    id: 'PR_LIST',
    text: 'MATCH (p:Process) RETURN p.id, p.processType, p.stepCount, p.communities, p.entryPointId, p.terminalId, p.label ORDER BY p.stepCount DESC, p.id SKIP {{offset}} LIMIT {{limit}}',
    slots: { offset: 'int', limit: 'int' },
    columns: [
      { header: 'p.id', type: 'string' },
      { header: 'p.processType', type: 'string' },
      { header: 'p.stepCount', type: 'int' },
      { header: 'p.communities', type: 'jsonArray' },
      { header: 'p.entryPointId', type: 'nullableString' },
      { header: 'p.terminalId', type: 'nullableString' },
      { header: 'p.label', type: 'string' }
    ],
    freeTextColumn: true,
    verifiedByCr: true
  },
  PR_COUNT: {
    id: 'PR_COUNT',
    text: 'MATCH (p:Process) RETURN count(*) AS n',
    slots: {},
    columns: [{ header: 'n', type: 'int' }],
    verifiedByCr: true
  },
  PR_ONE: {
    id: 'PR_ONE',
    text: 'MATCH (p:Process) WHERE p.id = {{id}} RETURN p.id, p.processType, p.stepCount, p.communities, p.entryPointId, p.terminalId, p.label',
    slots: { id: 'string' },
    columns: [
      { header: 'p.id', type: 'string' },
      { header: 'p.processType', type: 'string' },
      { header: 'p.stepCount', type: 'int' },
      { header: 'p.communities', type: 'jsonArray' },
      { header: 'p.entryPointId', type: 'nullableString' },
      { header: 'p.terminalId', type: 'nullableString' },
      { header: 'p.label', type: 'string' }
    ],
    freeTextColumn: true,
    verifiedByCr: true
  },
  PR_STEPS: {
    id: 'PR_STEPS',
    text: "MATCH (s)-[r:CodeRelation {type:'STEP_IN_PROCESS'}]->(p:Process) WHERE p.id = {{id}} RETURN s.id, s.name, s.filePath, label(s), s.startLine, s.endLine, r.step ORDER BY r.step LIMIT {{limit}}",
    slots: { id: 'string', limit: 'int' },
    columns: [
      { header: 's.id', type: 'string' },
      { header: 's.name', type: 'string' },
      { header: 's.filePath', type: 'string' },
      { header: 'label(s)', type: 'string' },
      { header: 's.startLine', type: 'int' },
      { header: 's.endLine', type: 'int' },
      { header: 'r.step', type: 'int' }
    ],
    freeTextColumn: true, // wait, name is string but maybe some spaces, but wait, freeTextColumn usually at the end. Here it's not at the end. So no freeTextColumn. Or put name at the end? The CR says `n.name` at the end for NODES_BY_ID. In PR_STEPS name is not at the end. Let's not use freeTextColumn for now, or just `name` isn't very long.
    verifiedByCr: true
  },
  NODES_BY_ID: {
    id: 'NODES_BY_ID',
    text: 'MATCH (n) WHERE n.id IN {{ids}} RETURN n.id, n.filePath, label(n), n.startLine, n.endLine, n.name',
    slots: { ids: 'stringList' },
    columns: [
      { header: 'n.id', type: 'string' },
      { header: 'n.filePath', type: 'string' },
      { header: 'label(n)', type: 'string' },
      { header: 'n.startLine', type: 'nullableString' }, // could be null if File
      { header: 'n.endLine', type: 'nullableString' },
      { header: 'n.name', type: 'string' }
    ],
    freeTextColumn: true,
    verifiedByCr: true
  },
  STEP_EDGES: {
    id: 'STEP_EDGES',
    text: 'MATCH (a)-[e:CodeRelation]->(b) WHERE a.id IN {{ids}} AND b.id IN {{ids}} AND e.type IN {{kinds}} RETURN a.id, b.id, e.type, e.confidence, e.reason',
    slots: { ids: 'stringList', kinds: 'kindList' },
    columns: [
      { header: 'a.id', type: 'string' },
      { header: 'b.id', type: 'string' },
      { header: 'e.type', type: 'string' },
      { header: 'e.confidence', type: 'float' },
      { header: 'e.reason', type: 'nullableString' }
    ],
    freeTextColumn: true,
    verifiedByCr: true
  },
  MEMBER_CLUSTER: {
    id: 'MEMBER_CLUSTER',
    text: "MATCH (s)-[:CodeRelation {type:'MEMBER_OF'}]->(c:Community) WHERE s.id IN {{ids}} RETURN s.id, c.id",
    slots: { ids: 'stringList' },
    columns: [
      { header: 's.id', type: 'string' },
      { header: 'c.id', type: 'string' }
    ],
    verifiedByCr: false
  },
  SG_EDGES_AROUND: {
    id: 'SG_EDGES_AROUND',
    text: 'MATCH (a)-[e:CodeRelation]->(b) WHERE (a.id IN {{frontier}} OR b.id IN {{frontier}}) AND e.type IN {{kinds}} RETURN a.id, b.id, e.type, e.confidence, e.reason LIMIT {{limit}}',
    slots: { frontier: 'stringList', kinds: 'kindList', limit: 'int' },
    columns: [
      { header: 'a.id', type: 'string' },
      { header: 'b.id', type: 'string' },
      { header: 'e.type', type: 'string' },
      { header: 'e.confidence', type: 'float' },
      { header: 'e.reason', type: 'nullableString' }
    ],
    freeTextColumn: true,
    verifiedByCr: false
  },
  SG_CLUSTER_MEMBERS: {
    id: 'SG_CLUSTER_MEMBERS',
    text: "MATCH (s)-[:CodeRelation {type:'MEMBER_OF'}]->(c:Community) WHERE c.id = {{id}} RETURN s.id, s.filePath, label(s), s.startLine, s.endLine, s.name LIMIT {{limit}}",
    slots: { id: 'string', limit: 'int' },
    columns: [
      { header: 's.id', type: 'string' },
      { header: 's.filePath', type: 'string' },
      { header: 'label(s)', type: 'string' },
      { header: 's.startLine', type: 'nullableString' },
      { header: 's.endLine', type: 'nullableString' },
      { header: 's.name', type: 'string' }
    ],
    freeTextColumn: true,
    verifiedByCr: true
  },
  SG_FILE_SYMBOLS: {
    id: 'SG_FILE_SYMBOLS',
    text: 'MATCH (n) WHERE n.filePath = {{path}} RETURN n.id, label(n), n.startLine, n.endLine, n.name ORDER BY n.startLine LIMIT {{limit}}',
    slots: { path: 'string', limit: 'int' },
    columns: [
      { header: 'n.id', type: 'string' },
      { header: 'label(n)', type: 'string' },
      { header: 'n.startLine', type: 'nullableString' },
      { header: 'n.endLine', type: 'nullableString' },
      { header: 'n.name', type: 'string' }
    ],
    freeTextColumn: true,
    verifiedByCr: true
  },
  FILE_SYMBOLS_BATCH: {
    id: 'FILE_SYMBOLS_BATCH',
    text: 'MATCH (n) WHERE n.filePath IN {{paths}} RETURN n.filePath, n.id, label(n), n.startLine, n.endLine, n.name',
    slots: { paths: 'stringList' },
    columns: [
      { header: 'n.filePath', type: 'string' },
      { header: 'n.id', type: 'string' },
      { header: 'label(n)', type: 'string' },
      { header: 'n.startLine', type: 'nullableString' },
      { header: 'n.endLine', type: 'nullableString' },
      { header: 'n.name', type: 'string' }
    ],
    freeTextColumn: true,
    verifiedByCr: false
  },
  SYMBOL_FLOWS: {
    id: 'SYMBOL_FLOWS',
    text: "MATCH (s)-[r:CodeRelation {type:'STEP_IN_PROCESS'}]->(p:Process) WHERE s.id IN {{ids}} RETURN s.id, p.id, p.stepCount, r.step, p.label",
    slots: { ids: 'stringList' },
    columns: [
      { header: 's.id', type: 'string' },
      { header: 'p.id', type: 'string' },
      { header: 'p.stepCount', type: 'int' },
      { header: 'r.step', type: 'int' },
      { header: 'p.label', type: 'string' }
    ],
    freeTextColumn: true,
    verifiedByCr: true
  },
  RT_LIST: {
    id: 'RT_LIST',
    text: "MATCH (h)-[e:CodeRelation]->(r:Route) WHERE e.type IN ['HANDLES_ROUTE','FETCHES'] RETURN r.id, r.method, r.filePath, e.type, h.id, e.confidence, r.name ORDER BY r.id SKIP {{offset}} LIMIT {{limit}}",
    slots: { offset: 'int', limit: 'int' },
    columns: [
      { header: 'r.id', type: 'string' },
      { header: 'r.method', type: 'nullableString' },
      { header: 'r.filePath', type: 'string' },
      { header: 'e.type', type: 'string' },
      { header: 'h.id', type: 'string' },
      { header: 'e.confidence', type: 'float' },
      { header: 'r.name', type: 'string' }
    ],
    freeTextColumn: true,
    verifiedByCr: false
  },
  RT_COUNTS: {
    id: 'RT_COUNTS',
    text: "MATCH (h)-[e:CodeRelation]->(r:Route) WHERE e.type IN ['HANDLES_ROUTE','FETCHES'] RETURN e.type, label(h), count(*) AS n",
    slots: {},
    columns: [
      { header: 'e.type', type: 'string' },
      { header: 'label(h)', type: 'string' },
      { header: 'n', type: 'int' }
    ],
    verifiedByCr: true
  }
}
