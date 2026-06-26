import type {
	Icon,
	ICredentialTestRequest,
	ICredentialType,
	INodeProperties,
} from 'n8n-workflow';

export class QbitBridgeApi implements ICredentialType {
	name = 'qbitBridgeApi';
	displayName = 'qBit Bridge API';
	documentationUrl = 'https://github.com/wyvernzora/qbit-bridge';
	icon: Icon = {
		light: 'file:qbittorrent-light.svg',
		dark: 'file:qbittorrent-dark.svg',
	};

	properties: INodeProperties[] = [
		{
			displayName: 'Base URL',
			name: 'baseUrl',
			type: 'string',
			default: 'http://qbit-bridge:8080',
			placeholder: 'http://qbit-bridge:8080',
			required: true,
			description: 'Base URL of qbit-bridge running with HTTP transport enabled',
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
