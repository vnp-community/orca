import fs from 'fs/promises';
import path from 'path';
import { exec } from 'child_process';
import os from 'os';
import { promisify } from 'util';

const execAsync = promisify(exec);

async function main() {
    const tmpDir = await fs.mkdtemp(path.join(os.tmpdir(), 'gitnexus-fixture-'));
    console.log(`Created temp dir: ${tmpDir}`);
    
    // Create Go+TS repo
    await fs.mkdir(path.join(tmpDir, 'internal/domain'), { recursive: true });
    await fs.mkdir(path.join(tmpDir, 'internal/usecase'), { recursive: true });
    await fs.mkdir(path.join(tmpDir, 'internal/usecase/usecasetest'), { recursive: true });
    await fs.mkdir(path.join(tmpDir, 'internal/adapter/a'), { recursive: true });
    await fs.mkdir(path.join(tmpDir, 'internal/adapter/b'), { recursive: true });
    
    // Create some files
    await execAsync('git init', { cwd: tmpDir });
    await execAsync('git config user.name "Test"', { cwd: tmpDir });
    await execAsync('git config user.email "test@example.com"', { cwd: tmpDir });
    await fs.writeFile(path.join(tmpDir, 'go.mod'), 'module sample\n\ngo 1.20\n');
    await fs.writeFile(path.join(tmpDir, 'internal/domain/entity.go'), 'package domain\n\ntype Entity struct {}\n');
    await fs.writeFile(path.join(tmpDir, 'internal/usecase/usecase.go'), 'package usecase\n\nimport "sample/internal/adapter/a"\n\nfunc Do() {\n\ta.Call()\n}\n');
    await fs.writeFile(path.join(tmpDir, 'internal/adapter/a/a1.go'), 'package a\n\nfunc Call() {}\n');
    await fs.writeFile(path.join(tmpDir, 'internal/adapter/a/a2.go'), 'package a\n\nfunc Call2() {}\n');
    await fs.writeFile(path.join(tmpDir, 'internal/usecase/usecasetest/mock.go'), 'package usecasetest\n\nfunc Mock() {}\n');
    await fs.writeFile(path.join(tmpDir, 'internal/usecase/usecase_test.go'), 'package usecase_test\n\nfunc TestDo() {}\n');
    await fs.writeFile(path.join(tmpDir, 'internal/unused.go'), 'package internal\n\nfunc UnusedFunc() {}\n');
    
    // Create TS cycle
    await fs.writeFile(path.join(tmpDir, 'a.ts'), 'import { b } from "./b";\nexport const a = b + 1;\n');
    await fs.writeFile(path.join(tmpDir, 'b.ts'), 'import { a } from "./a";\nexport const b = a + 1;\n');
    await fs.writeFile(path.join(tmpDir, 'package.json'), '{"name":"sample","version":"1.0.0"}\n');
    await fs.writeFile(path.join(tmpDir, 'tsconfig.json'), '{"compilerOptions":{"target":"es2020","module":"commonjs"}}\n');

    // Commit files
    await execAsync('git add .', { cwd: tmpDir });
    await execAsync('git commit -m "init"', { cwd: tmpDir });

    // Run gitnexus analyze
    console.log('Running gitnexus analyze...');
    try {
        const { stdout, stderr } = await execAsync('npx gitnexus analyze --index-only', { 
            cwd: tmpDir, 
            env: { ...process.env, GITNEXUS_WORKER_POOL_SIZE: '1' } 
        });
        console.log('analyze stdout:', stdout);
        console.log('analyze stderr:', stderr);
    } catch (e) {
        console.error('gitnexus analyze error:', e.message);
        if (e.stdout) console.error('stdout:', e.stdout);
        if (e.stderr) console.error('stderr:', e.stderr);
    }
    
    const fixturesDir = path.join(process.cwd(), 'agent/src/relay/__fixtures__/structural-facts');
    await fs.mkdir(fixturesDir, { recursive: true });
    
    // Check cycles
    try {
        const { stdout } = await execAsync(`npx gitnexus check --cycles --json -r "${tmpDir}"`, { cwd: tmpDir });
        await fs.writeFile(path.join(fixturesDir, 'cycles.json'), stdout);
    } catch (e) {
        if (e.stdout) {
            await fs.writeFile(path.join(fixturesDir, 'cycles.json'), e.stdout);
        } else {
            console.error('Error getting cycles', e);
        }
    }
    
    // Run cypher for layerImports
    const layerImportsCypher = `MATCH (f1:File)-[:CodeRelation {type: 'IMPORTS'}]->(f2:File) RETURN f1.filePath as fromFile, f2.filePath as toFile`;
    try {
        const { stdout } = await execAsync(`npx gitnexus cypher "${layerImportsCypher}" -r "${tmpDir}"`, { cwd: tmpDir });
        await fs.writeFile(path.join(fixturesDir, 'layerImports.md'), stdout);
    } catch (e) {
        console.log("layer imports query failed", e.message);
    }

    const inDegreeCypher = `MATCH (f2:File)-[:CodeRelation {type: 'IMPORTS'}]->(f1:File) RETURN f1.filePath as file, count(f2) as inDegree`;
    try {
        const { stdout } = await execAsync(`npx gitnexus cypher "${inDegreeCypher}" -r "${tmpDir}"`, { cwd: tmpDir });
        await fs.writeFile(path.join(fixturesDir, 'importInDegree.md'), stdout);
    } catch (e) {
        console.log("indegree query failed", e.message);
    }

    const fileSizesCypher = `MATCH (f:File)-[:CodeRelation {type: 'CONTAINS'}]->(func:Function) RETURN f.filePath as file, count(func) as functions`;
    try {
        const { stdout } = await execAsync(`npx gitnexus cypher "${fileSizesCypher}" -r "${tmpDir}"`, { cwd: tmpDir });
        await fs.writeFile(path.join(fixturesDir, 'fileSizes.md'), stdout);
    } catch (e) {
        console.log("filesizes query failed", e.message);
    }

    const unusedExportsCypher = `MATCH (f:File)-[:CodeRelation {type: 'CONTAINS'}]->(func:Function) WHERE NOT ()-[:CodeRelation {type: 'CALLS'}]->(func) RETURN func.name as symbol, f.filePath as file`;
    try {
        const { stdout } = await execAsync(`npx gitnexus cypher "${unusedExportsCypher}" -r "${tmpDir}"`, { cwd: tmpDir });
        await fs.writeFile(path.join(fixturesDir, 'unusedExports.md'), stdout);
    } catch (e) {
        console.log("unused exports query failed", e.message);
    }

    // Write MANIFEST
    await fs.writeFile(path.join(fixturesDir, 'MANIFEST.json'), JSON.stringify({
        gitnexusVersion: "1.6.13-rc.85",
        argv: "capture-structural-facts-fixtures.mjs",
        hash: "dummy-hash"
    }, null, 2));

    console.log('Done.');
}

main().catch(console.error);
