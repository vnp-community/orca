const fromSeg = '/internal/domain/'
const toSeg = '/internal/adapter/'
const pathPrefixes = ['backend-go/services/']
let prefixClause = pathPrefixes.map(p => `a.filePath STARTS WITH '${p}'`).join(' OR ')
const query = `MATCH (a:File)-[r:CodeRelation {type:'IMPORTS'}]->(b:File) ` +
`WHERE a.filePath CONTAINS '${fromSeg}' ` +
`AND b.filePath CONTAINS '${toSeg}' ` +
`AND NOT a.filePath ENDS WITH '_test.go' ` +
`AND NOT a.filePath CONTAINS '/usecasetest/' ` +
`AND NOT a.filePath CONTAINS '/testutil/' ` +
`AND NOT a.filePath CONTAINS '/cmd/' ` +
`AND NOT a.filePath CONTAINS '/proto/gen/' ` +
`AND NOT a.filePath ENDS WITH '.pb.go' ` +
`AND (${prefixClause}) ` +
`RETURN a.filePath, b.filePath`
console.log(query.match(/[\x00-\x1F\x7F]/))
