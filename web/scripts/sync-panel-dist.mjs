import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const webRoot = path.resolve(__dirname, '..');
const sourceDir = path.join(webRoot, 'dist');
const targetDir = path.resolve(webRoot, '..', 'panel', 'web', 'dist');

if (!fs.existsSync(sourceDir)) {
  throw new Error(`source dist not found: ${sourceDir}`);
}

fs.rmSync(targetDir, { recursive: true, force: true });
fs.mkdirSync(path.dirname(targetDir), { recursive: true });
fs.cpSync(sourceDir, targetDir, { recursive: true });

console.log(`Synced frontend build from ${sourceDir} to ${targetDir}`);
