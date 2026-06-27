import type {
	IDataObject,
	IExecuteFunctions,
	IHttpRequestMethods,
	INodeExecutionData,
	INodeType,
	INodeTypeDescription,
} from 'n8n-workflow';

const CRED = 'qbitBridgeApi';

export class QbitBridge implements INodeType {
	description: INodeTypeDescription = {
		displayName: 'qBit Bridge',
		name: 'qbitBridge',
		icon: {
			light: 'file:qbittorrent-dark.svg',
			dark: 'file:qbittorrent-light.svg',
		},
		group: ['transform'],
		version: 1,
		subtitle: '={{$parameter["operation"] + " (" + $parameter["resource"] + ")"}}',
		description: 'Drive qbit-bridge download workflows',
		defaults: { name: 'qBit Bridge' },
		inputs: ['main'],
		outputs: ['main'],
		credentials: [{ name: CRED, required: true }],
		properties: [
			{
				displayName: 'Resource',
				name: 'resource',
				type: 'options',
				noDataExpression: true,
				options: [{ name: 'Download', value: 'download' }],
				default: 'download',
			},
			{
				displayName: 'Operation',
				name: 'operation',
				type: 'options',
				noDataExpression: true,
				displayOptions: { show: { resource: ['download'] } },
				options: [
					{ name: 'Add', value: 'add', action: 'Add a download' },
					{ name: 'Get', value: 'get', action: 'Get a download' },
					{ name: 'List', value: 'list', action: 'List downloads' },
					{ name: 'Remove', value: 'remove', action: 'Remove a download' },
				],
				default: 'list',
			},
			{
				displayName: 'Magnet',
				name: 'magnet',
				type: 'string',
				default: '={{$json.magnet}}',
				required: true,
				description: 'Magnet URI with xt=urn:btih:<hash>',
				displayOptions: { show: { resource: ['download'], operation: ['add'] } },
			},
			{
				displayName: 'Tags',
				name: 'addTags',
				type: 'string',
				default: '={{$json.tags}}',
				description: 'Comma-separated tags to apply on add',
				displayOptions: { show: { resource: ['download'], operation: ['add'] } },
			},
			{
				displayName: 'Destination',
				name: 'destination',
				type: 'string',
				default: '={{$json.destination}}',
				description: 'Destination alias, for example kura-inbox or kura-inbox:show-name',
				displayOptions: { show: { resource: ['download'], operation: ['add'] } },
			},
			{
				displayName: 'Rename',
				name: 'rename',
				type: 'string',
				default: '={{$json.rename}}',
				description: 'Optional display name override in qBittorrent',
				displayOptions: { show: { resource: ['download'], operation: ['add'] } },
			},
			{
				displayName: 'Hash',
				name: 'hash',
				type: 'string',
				default: '={{$json.hash}}',
				required: true,
				description: 'Download hash',
				displayOptions: { show: { resource: ['download'], operation: ['get', 'remove'] } },
			},
			{
				displayName: 'States',
				name: 'states',
				type: 'string',
				default: '',
				placeholder: 'downloading,seeding',
				description: 'Comma-separated normalized state filters',
				displayOptions: { show: { resource: ['download'], operation: ['list'] } },
			},
			{
				displayName: 'Tags',
				name: 'filterTags',
				type: 'string',
				default: '',
				placeholder: 'tvdb:*,weekly',
				description: 'Comma-separated tag patterns using Go path.Match glob syntax',
				displayOptions: { show: { resource: ['download'], operation: ['list'] } },
			},
			{
				displayName: 'Hashes',
				name: 'hashes',
				type: 'string',
				default: '',
				description: 'Comma-separated hashes to list',
				displayOptions: { show: { resource: ['download'], operation: ['list'] } },
			},
			{
				displayName: 'Sort',
				name: 'sort',
				type: 'options',
				options: [
					{ name: 'Added On Desc', value: 'added_on_desc' },
					{ name: 'Added On Asc', value: 'added_on_asc' },
					{ name: 'Download Speed Desc', value: 'dlspeed_desc' },
					{ name: 'ETA Asc', value: 'eta_asc' },
					{ name: 'Name Asc', value: 'name_asc' },
					{ name: 'Name Desc', value: 'name_desc' },
					{ name: 'Progress Asc', value: 'progress_asc' },
					{ name: 'Progress Desc', value: 'progress_desc' },
					{ name: 'Ratio Desc', value: 'ratio_desc' },
					{ name: 'Size Asc', value: 'size_asc' },
					{ name: 'Size Desc', value: 'size_desc' },
				],
				default: 'added_on_desc',
				displayOptions: { show: { resource: ['download'], operation: ['list'] } },
			},
			{
				displayName: 'Limit',
				name: 'limit',
				type: 'number',
				typeOptions: { minValue: 0, maxValue: 200 },
				default: 50,
				description: 'Maximum downloads to return',
				displayOptions: { show: { resource: ['download'], operation: ['list'] } },
			},
			{
				displayName: 'Offset',
				name: 'offset',
				type: 'number',
				typeOptions: { minValue: 0 },
				default: 0,
				description: 'Pagination offset',
				displayOptions: { show: { resource: ['download'], operation: ['list'] } },
			},
			{
				displayName: 'Include Fields',
				name: 'includeFields',
				type: 'multiOptions',
				options: [
					{ name: 'All Field-Level Values', value: 'all' },
					{ name: 'Completion On', value: 'completion_on' },
					{ name: 'Content Path', value: 'content_path' },
					{ name: 'Files', value: 'files' },
					{ name: 'Last Activity', value: 'last_activity' },
					{ name: 'Magnet URI', value: 'magnet_uri' },
					{ name: 'Peers', value: 'peers' },
					{ name: 'Private', value: 'private' },
					{ name: 'Save Path', value: 'save_path' },
					{ name: 'Seeding Time', value: 'seeding_time' },
					{ name: 'Seeds', value: 'seeds' },
					{ name: 'Seeds Incomplete', value: 'seeds_incomplete' },
					{ name: 'Total Downloaded', value: 'total_downloaded' },
					{ name: 'Total Size', value: 'total_size' },
					{ name: 'Total Uploaded', value: 'total_uploaded' },
					{ name: 'Tracker Count', value: 'tracker_count' },
					{ name: 'Trackers', value: 'trackers' },
				],
				default: [],
				description: 'Optional projection fields. Trackers and files can make extra qBittorrent calls.',
				displayOptions: { show: { resource: ['download'], operation: ['list', 'get'] } },
			},
		],
	};

	async execute(this: IExecuteFunctions): Promise<INodeExecutionData[][]> {
		const operation = this.getNodeParameter('operation', 0) as string;
		const credentials = await this.getCredentials(CRED);
		const call = callFactory(this, credentials);

		if (operation === 'list') {
			const result = await call('GET', `/api/v1/downloads?${listQuery(this).toString()}`);
			return [arrayField(result, 'downloads').map((download) => ({ json: download }))];
		}

		const items = this.getInputData();
		const out: INodeExecutionData[] = [];
		for (let i = 0; i < items.length; i++) {
			try {
				if (operation === 'add') {
					const result = await call('POST', '/api/v1/downloads', addBody(this, i));
					out.push({ json: result, pairedItem: { item: i } });
					continue;
				}

				const hash = this.getNodeParameter('hash', i) as string;
				if (operation === 'get') {
					const query = includeFieldsQuery(this, i);
					const suffix = query.toString();
					const path = `/api/v1/downloads/${encodeURIComponent(hash)}${suffix === '' ? '' : `?${suffix}`}`;
					const result = await call('GET', path);
					out.push({ json: result, pairedItem: { item: i } });
					continue;
				}

				const result = await call('DELETE', `/api/v1/downloads/${encodeURIComponent(hash)}`);
				out.push({ json: result, pairedItem: { item: i } });
			} catch (error) {
				if (this.continueOnFail()) {
					out.push({ json: { error: (error as Error).message }, pairedItem: { item: i } });
					continue;
				}
				throw error;
			}
		}
		return [out];
	}
}

type HTTPCall = (method: IHttpRequestMethods, path: string, body?: IDataObject) => Promise<IDataObject>;

function callFactory(ctx: IExecuteFunctions, credentials: IDataObject): HTTPCall {
	const baseUrl = String(credentials.baseUrl).replace(/\/+$/, '');
	return (method, path, body) =>
		ctx.helpers.httpRequest({
			method,
			url: `${baseUrl}${path}`,
			body,
			json: true,
		}) as Promise<IDataObject>;
}

function addBody(ctx: IExecuteFunctions, itemIndex: number): IDataObject {
	return dropEmpty({
		magnet: ctx.getNodeParameter('magnet', itemIndex),
		tags: csv(ctx.getNodeParameter('addTags', itemIndex) as string),
		destination: ctx.getNodeParameter('destination', itemIndex),
		rename: ctx.getNodeParameter('rename', itemIndex),
	});
}

function listQuery(ctx: IExecuteFunctions): URLSearchParams {
	const query = new URLSearchParams();
	for (const state of csv(ctx.getNodeParameter('states', 0) as string)) query.append('states', state);
	for (const tag of csv(ctx.getNodeParameter('filterTags', 0) as string)) query.append('tags', tag);
	for (const hash of csv(ctx.getNodeParameter('hashes', 0) as string)) query.append('hashes', hash);
	for (const field of ctx.getNodeParameter('includeFields', 0) as string[]) {
		query.append('include_fields', field);
	}
	query.set('sort', ctx.getNodeParameter('sort', 0) as string);
	query.set('limit', String(ctx.getNodeParameter('limit', 0)));
	query.set('offset', String(ctx.getNodeParameter('offset', 0)));
	return query;
}

function includeFieldsQuery(ctx: IExecuteFunctions, itemIndex: number): URLSearchParams {
	const query = new URLSearchParams();
	for (const field of ctx.getNodeParameter('includeFields', itemIndex) as string[]) {
		query.append('include_fields', field);
	}
	return query;
}

function csv(value: string): string[] {
	return value
		.split(',')
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

function dropEmpty(obj: IDataObject): IDataObject {
	const out: IDataObject = {};
	for (const [key, value] of Object.entries(obj)) {
		if (value === undefined || value === null || value === '') continue;
		if (Array.isArray(value) && value.length === 0) continue;
		out[key] = value as IDataObject[string];
	}
	return out;
}
