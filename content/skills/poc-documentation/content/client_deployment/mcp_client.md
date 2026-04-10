## Prerequisites

- Aembit MCP Identity Gateway endpoint URL (coordinate with Aembit Solutions Engineer): `{{MCP_GATEWAY_ENDPOINT}}`

## Deployment

1. [Configure a Claude Custom Connector for the MCP Gateway in Claude Web](https://support.claude.com/en/articles/11175166-get-started-with-custom-connectors-using-remote-mcp):
   - The MCP Server URL is `https://{{MCP_GATEWAY_ENDPOINT}}/mcp` 
   - Advanced Settings are not needed

2. Connect to the MCP Gateway Connector:
   - Open Claude connector list → **Settings → Connectors**
   - Click Connect on the Connector created in step 1
   - Log in with the configured Identity Provider and authorize MCP Servers.
   - After clicking return to client Claude can invoke MCP tools on the gateway

## Verification

- In Claude Desktop or Claude Web, send a prompt that requires MCP access (e.g., "List my Box files") and confirm Claude returns results from the MCP server
- The MCP server connection indicator shows as active in Claude's Connector settings
- Navigate to **Reporting** in the Aembit Management Console and confirm Access Authorization Events appear

## Troubleshooting

- **MCP server shows as disconnected in Claude:** The MCP Gateway is configured, but not connected. Verify the URL matches exactly what is provided by your Aembit Solutions Architect and check the Access Authorization Events for errors.
- **Claude can connect but MCP tools return errors:** Policy 2 (user to MCP server) is not configured or the Credential Provider is inactive. Check the mcp_server server integration module is also completed and check the Access Authorization Events for connections to the MCP Server
- **Claude Enterprise: MCP server not visible to users:** The MCP server was added but not enabled for the workspace. Check the **Settings → Connectors** page in Claude Web
