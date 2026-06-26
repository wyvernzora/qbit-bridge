import type {
	Icon,
	ICredentialTestRequest,
	ICredentialType,
	INodeProperties,
} from 'n8n-workflow';

export class QBittorrentMcpApi implements ICredentialType {
	name = 'qbittorrentMcpApi';
	displayName = 'qBittorrent MCP API';
	documentationUrl = 'https://github.com/wyvernzora/qbittorrent-mcp';
	icon: Icon = {
		light: 'file:qbittorrent-light.svg',
		dark: 'file:qbittorrent-dark.svg',
	};

	properties: INodeProperties[] = [
		{
			displayName: 'Base URL',
			name: 'baseUrl',
			type: 'string',
			default: 'http://qbit-mcp:8080',
			placeholder: 'http://qbit-mcp:8080',
			required: true,
			description: 'Base URL of qbittorrent-mcp running with HTTP transport enabled',
		},
	];

	test: ICredentialTestRequest = {
		request: {
			baseURL: '={{$credentials.baseUrl}}',
			url: '/healthz',
			method: 'GET',
		},
	};
}
