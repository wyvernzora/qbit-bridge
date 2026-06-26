import { access, cp, mkdir, readdir } from 'node:fs/promises';
import { join } from 'node:path';

const ICONS = ['qbittorrent-light.svg', 'qbittorrent-dark.svg'];
const NODES = 'dist/nodes';
const CREDENTIALS = 'dist/credentials';

for (const icon of ICONS) {
	await access(icon);
}

let n = 0;
for (const entry of await readdir(NODES, { withFileTypes: true })) {
	if (!entry.isDirectory()) continue;
	const dir = join(NODES, entry.name);
	await mkdir(dir, { recursive: true });
	for (const icon of ICONS) {
		await cp(icon, join(dir, icon));
		n++;
	}
}

await mkdir(CREDENTIALS, { recursive: true });
for (const icon of ICONS) {
	await cp(icon, join(CREDENTIALS, icon));
	n++;
}

console.log(`copy-assets: placed ${n} qBittorrent icon file(s)`);
