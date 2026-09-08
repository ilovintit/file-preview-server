import { createHash } from 'node:crypto';
import { mkdir, writeFile } from 'node:fs/promises';
const base='https://raw.githubusercontent.com/apache/tika/e918be5e9d1031a373528215a32ea5a1e6488624/tika-parsers/tika-parsers-standard/tika-parsers-standard-integration-tests/src/test/resources/test-documents/';
await mkdir('.cache/office-xml-reference',{recursive:true});
for(const [name,hash] of [['testStarOffice-6.0-writer.sxw','a515c2e4eff54f6cc1ae86678146f4649911455e'],['testStarOffice-6.0-writer-template.stw','be448ade48ec8d4f34173dba862555ce74e34059']]){
  const response=await fetch(base+name,{signal:AbortSignal.timeout(30000)});
  if(!response.ok)throw new Error(`reference download ${response.status}`);
  const data=Buffer.from(await response.arrayBuffer());
  if(createHash('sha1').update(`blob ${data.length}\0`).update(data).digest('hex')!==hash)throw new Error('reference hash mismatch');
  await writeFile('.cache/office-xml-reference/'+name,data);
}
