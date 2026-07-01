import assert from 'node:assert/strict';
import { createRequire } from 'node:module';

const require = createRequire(import.meta.url);
const { QbitBridge } = require('../dist/nodes/QbitBridge/QbitBridge.node.js');
const { QbitBridgeDownloadFinishedTrigger } = require('../dist/nodes/QbitBridge/QbitBridgeDownloadFinishedTrigger.node.js');

await testActionOperations();
await testTriggerPoll();

async function testActionOperations() {
	await assertAction({
		name: 'list',
		params: {
			operation: 'list',
			filterTags: 'tvdb:*, weekly',
			notTags: 'require-review',
			hashes: 'aaa,bbb',
			includeFields: ['save_path'],
			sort: 'added_on_desc',
			limit: 50,
			offset: 0,
		},
		response: { downloads: [{ hash: 'aaa' }] },
		assertResult(result, calls) {
			assert.deepEqual(result, [[{ json: { hash: 'aaa' } }]]);
			const call = only(calls);
			assert.equal(call.method, 'GET');
			const url = new URL(call.url);
			assert.equal(url.pathname, '/api/v1/downloads');
			assert.deepEqual(url.searchParams.getAll('tags'), ['tvdb:*', 'weekly']);
			assert.deepEqual(url.searchParams.getAll('not_tags'), ['require-review']);
			assert.deepEqual(url.searchParams.getAll('hashes'), ['aaa', 'bbb']);
			assert.deepEqual(url.searchParams.getAll('include_fields'), ['save_path']);
			assert.equal(url.searchParams.get('sort'), 'added_on_desc');
			assert.equal(url.searchParams.get('limit'), '50');
			assert.equal(url.searchParams.get('offset'), '0');
		},
	});

	await assertAction({
		name: 'add',
		params: {
			operation: 'add',
			magnet: 'magnet:?xt=urn:btih:aaa',
			addTags: 'weekly, require-review',
			destination: 'kura-inbox',
			rename: 'Show 01',
		},
		response: { hash: 'aaa', accepted: true },
		assertResult(result, calls) {
			assert.deepEqual(result[0][0].json, { hash: 'aaa', accepted: true });
			assert.deepEqual(only(calls), {
				method: 'POST',
				url: 'http://bridge/api/v1/downloads',
				body: {
					magnet: 'magnet:?xt=urn:btih:aaa',
					tags: ['weekly', 'require-review'],
					destination: 'kura-inbox',
					rename: 'Show 01',
				},
				json: true,
			});
		},
	});

	await assertAction({
		name: 'get',
		params: {
			operation: 'get',
			hash: 'a/b?',
			includeFields: ['content_path'],
		},
		response: { hash: 'a/b?' },
		assertResult(_, calls) {
			assert.equal(only(calls).method, 'GET');
			const url = new URL(only(calls).url);
			assert.equal(url.pathname, '/api/v1/downloads/a%2Fb%3F');
			assert.deepEqual(url.searchParams.getAll('include_fields'), ['content_path']);
		},
	});

	await assertAction({
		name: 'remove',
		params: {
			operation: 'remove',
			hash: 'aaa',
		},
		response: { affected_count: 1, affected_hashes: ['aaa'] },
		assertResult(_, calls) {
			assert.deepEqual(only(calls), {
				method: 'DELETE',
				url: 'http://bridge/api/v1/downloads/aaa',
				body: undefined,
				json: true,
			});
		},
	});

	await assertAction({
		name: 'updateTags',
		params: {
			operation: 'updateTags',
			hash: 'aaa',
			addTags: 'require-review',
			removeTags: 'auto-adopt, weekly',
		},
		response: { affected_count: 1, affected_hashes: ['aaa'] },
		assertResult(_, calls) {
			assert.deepEqual(only(calls), {
				method: 'PUT',
				url: 'http://bridge/api/v1/downloads/aaa/tags',
				body: { add: ['require-review'], remove: ['auto-adopt', 'weekly'] },
				json: true,
			});
		},
	});
}

async function assertAction({ params, response, assertResult }) {
	const calls = [];
	const ctx = {
		getNodeParameter: (name) => params[name],
		getCredentials: async () => ({ baseUrl: 'http://bridge/' }),
		getInputData: () => [{ json: {} }],
		continueOnFail: () => false,
		helpers: {
			httpRequest: async (request) => {
				calls.push(request);
				return response;
			},
		},
	};
	const result = await QbitBridge.prototype.execute.call(ctx);
	assertResult(result, calls);
}

async function testTriggerPoll() {
	const calls = [];
	const staticData = {};
	const pages = [
		{
			has_more: true,
			downloads: [
				{ hash: 'done1', completion_on: 1, state: 'uploading', progress: 1, dlspeed_bytes_per_sec: 0, eta_seconds: null, save_path: 'anime', content_path: 'anime:01.mkv' },
				{ hash: 'incomplete', completion_on: 0, progress: 0.5 },
			],
		},
		{
			has_more: false,
			downloads: [
				{ hash: 'done2', completion_on: 0, progress: 1, upspeed_bytes_per_sec: 0, save_path: 'anime', content_path: 'anime:02.mkv' },
			],
		},
	];
	const ctx = {
		getNodeParameter: (name) => ({
			filterTags: 'tvdb:*',
			notTags: 'require-review',
			leaseMinutes: 5,
			limit: 2,
		})[name],
		getCredentials: async () => ({ baseUrl: 'http://bridge/' }),
		getWorkflowStaticData: () => staticData,
		helpers: {
			httpRequest: async (request) => {
				calls.push(request);
				return pages.shift() ?? { has_more: false, downloads: [] };
			},
		},
	};

	const first = await QbitBridgeDownloadFinishedTrigger.prototype.poll.call(ctx);
	assert.equal(first[0].length, 2);
	assert.deepEqual(first[0].map((item) => item.json.hash), ['done1', 'done2']);
	assert.equal(first[0][0].json.save_path, 'anime');
	assert.equal(first[0][0].json.content_path, 'anime:01.mkv');
	assert.equal('state' in first[0][0].json, false);
	assert.equal('progress' in first[0][0].json, false);
	assert.equal('dlspeed_bytes_per_sec' in first[0][0].json, false);
	assert.equal('upspeed_bytes_per_sec' in first[0][1].json, false);
	assert.equal('eta_seconds' in first[0][0].json, false);

	assert.equal(calls.length, 2);
	const firstURL = new URL(calls[0].url);
	assert.equal(calls[0].method, 'GET');
	assert.equal(firstURL.pathname, '/api/v1/downloads');
	assert.deepEqual(firstURL.searchParams.getAll('tags'), ['tvdb:*']);
	assert.deepEqual(firstURL.searchParams.getAll('not_tags'), ['require-review']);
	assert.deepEqual(firstURL.searchParams.getAll('include_fields'), ['completion_on', 'save_path', 'content_path']);
	assert.equal(firstURL.searchParams.get('limit'), '200');
	assert.equal(firstURL.searchParams.get('offset'), '0');
	assert.equal(new URL(calls[1].url).searchParams.get('offset'), '2');

	pages.push({ has_more: false, downloads: [{ hash: 'done1', completion_on: 1 }, { hash: 'done2', progress: 1 }] });
	const second = await QbitBridgeDownloadFinishedTrigger.prototype.poll.call(ctx);
	assert.equal(second, null);
}

function only(values) {
	assert.equal(values.length, 1);
	return values[0];
}
