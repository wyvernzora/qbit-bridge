import type {
	IDataObject,
	IHttpRequestMethods,
	INodeExecutionData,
	INodeType,
	INodeTypeDescription,
	IPollFunctions,
} from 'n8n-workflow';

const CRED = 'qbitBridgeApi';
const pageLimit = 200;

export class QbitBridgeDownloadFinishedTrigger implements INodeType {
	description: INodeTypeDescription = {
		displayName: 'qBit Bridge Download Finished Trigger',
		name: 'qbitBridgeTrigger',
		icon: {
			light: 'file:qbittorrent-dark.svg',
			dark: 'file:qbittorrent-light.svg',
		},
		group: ['trigger'],
		version: 1,
		subtitle: '={{"tags: " + ($parameter["filterTags"] || "any") + ($parameter["notTags"] ? ", not: " + $parameter["notTags"] : "")}}',
		description: 'Emit completed qbit-bridge downloads with a retry lease',
		defaults: { name: 'qBit Bridge Download Finished Trigger' },
		polling: true,
		inputs: [],
		outputs: ['main'],
		credentials: [{ name: CRED, required: true }],
		properties: [
			{
				displayName: 'Tags',
				name: 'filterTags',
				type: 'string',
				default: '',
				placeholder: 'tvdb:*,weekly',
				description: 'Comma-separated tag patterns using Go path.Match glob syntax',
			},
			{
				displayName: 'Not Tags',
				name: 'notTags',
				type: 'string',
				default: '',
				placeholder: 'review,adopt:blocked',
				description: 'Comma-separated tag patterns to exclude',
			},
			{
				displayName: 'Lease Minutes',
				name: 'leaseMinutes',
				type: 'number',
				typeOptions: { minValue: 1 },
				default: 5,
				description: 'How long to suppress a completed download after emitting it',
			},
			{
				displayName: 'Limit',
				name: 'limit',
				type: 'number',
				typeOptions: { minValue: 1 },
				default: 10,
				description: 'Maximum completed downloads to emit per poll',
			},
		],
	};

	async poll(this: IPollFunctions): Promise<INodeExecutionData[][] | null> {
		const credentials = await this.getCredentials(CRED);
		const call = callFactory(this, credentials);
		const out = await pollCompletedDownloads(this, call);
		return out.length === 0 ? null : [out];
	}
}

type HTTPCall = (method: IHttpRequestMethods, path: string) => Promise<IDataObject>;

function callFactory(ctx: IPollFunctions, credentials: IDataObject): HTTPCall {
	const baseUrl = String(credentials.baseUrl).replace(/\/+$/, '');
	return (method, path) =>
		ctx.helpers.httpRequest({
			method,
			url: `${baseUrl}${path}`,
			json: true,
		}) as Promise<IDataObject>;
}

async function pollCompletedDownloads(ctx: IPollFunctions, call: HTTPCall): Promise<INodeExecutionData[]> {
	const now = Date.now();
	const leaseMs = Math.max(1, ctx.getNodeParameter('leaseMinutes', 5) as number) * 60_000;
	const limit = Math.max(1, ctx.getNodeParameter('limit', 10) as number);
	const downloads = await listDownloads(ctx, call);
	const completed = downloads.filter(isCompletedDownload);
	const completedHashes = new Set(completed.map((download) => String(download.hash)));
	const leases = leaseTable(ctx.getWorkflowStaticData('node'));
	const out: INodeExecutionData[] = [];

	for (const download of completed) {
		const hash = String(download.hash);
		if ((leases[hash] ?? 0) > now) continue;
		leases[hash] = now + leaseMs;
		out.push({ json: triggerOutput(download) });
		if (out.length >= limit) break;
	}

	for (const hash of Object.keys(leases)) {
		if (!completedHashes.has(hash)) delete leases[hash];
	}

	return out;
}

async function listDownloads(ctx: IPollFunctions, call: HTTPCall): Promise<IDataObject[]> {
	const all: IDataObject[] = [];
	for (let offset = 0; ; ) {
		const query = listQuery(ctx, offset);
		const result = await call('GET', `/api/v1/downloads?${query.toString()}`);
		const page = arrayField(result, 'downloads');
		all.push(...page);
		if (result.has_more !== true || page.length === 0) return all;
		offset += page.length;
	}
}

function listQuery(ctx: IPollFunctions, offset: number): URLSearchParams {
	const query = new URLSearchParams();
	for (const tag of csv(ctx.getNodeParameter('filterTags', ''))) query.append('tags', tag);
	for (const tag of csv(ctx.getNodeParameter('notTags', ''))) query.append('not_tags', tag);
	query.append('include_fields', 'completion_on');
	query.append('include_fields', 'save_path');
	query.append('include_fields', 'content_path');
	query.set('limit', String(pageLimit));
	query.set('offset', String(offset));
	return query;
}

function isCompletedDownload(download: IDataObject): boolean {
	return Number(download.completion_on ?? 0) > 0 || Number(download.progress ?? 0) >= 1;
}

function triggerOutput(download: IDataObject): IDataObject {
	const out = { ...download };
	delete out.state;
	delete out.progress;
	delete out.dlspeed;
	delete out.dlspeed_bytes_per_sec;
	delete out.upspeed;
	delete out.upspeed_bytes_per_sec;
	delete out.eta_seconds;
	return out;
}

function leaseTable(staticData: IDataObject): Record<string, number> {
	if (!isObject(staticData.leases)) staticData.leases = {};
	return staticData.leases as Record<string, number>;
}

function csv(value: unknown): string[] {
	const values = Array.isArray(value) ? value : [value];
	return values
		.flatMap((part) => String(part ?? '').split(','))
		.map((part) => part.trim())
		.filter((part) => part !== '');
}

function arrayField(obj: IDataObject, key: string): IDataObject[] {
	const value = obj[key];
	return Array.isArray(value) ? (value.filter(isObject) as IDataObject[]) : [];
}

function isObject(value: unknown): value is IDataObject {
	return value !== null && typeof value === 'object' && !Array.isArray(value);
}
